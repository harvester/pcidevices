package deviceplugins

/* This file was part of the KubeVirt project, copied to this project
 * to get around private package issues.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2023 SUSE, LLC.
 *
 */

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"kubevirt.io/kubevirt/pkg/util"
	pluginapi "kubevirt.io/kubevirt/pkg/virt-handler/device-manager/deviceplugin/v1beta1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
)

const (
	vgpuPrefix = "MDEV_PCI_RESOURCE"
)

type VGPUDevicePlugin struct {
	devs         []*pluginapi.Device
	server       *grpc.Server
	socketPath   string
	stop         <-chan struct{}
	health       chan deviceHealth
	devicePath   string
	resourceName string
	done         chan struct{}
	deviceRoot   string
	initialized  bool
	lock         *sync.Mutex
	deregistered chan struct{}
	starter      *DeviceStarter
	ctx          context.Context
}

// Not adding more data to the struct, it's big enough already
func (dp *VGPUDevicePlugin) GetCount() int {
	var count int
	for _, dev := range dp.devs {
		if dev.Health == pluginapi.Healthy {
			count++
		}
	}
	logrus.Debugf("found device count %d for plugin %s", count, dp.resourceName)
	return count
}

func NewVGPUDevicePlugin(ctx context.Context, vGPUList []string, resourceName string) *VGPUDevicePlugin {
	serverSock := SocketPath(strings.Replace(resourceName, "/", "-", -1))

	devs := constructVGPUDPIdevices(vGPUList)
	dpi := &VGPUDevicePlugin{
		devs:         devs,
		socketPath:   serverSock,
		resourceName: resourceName,
		devicePath:   v1beta1.MdevRoot,
		deviceRoot:   util.HostRootMount,
		health:       make(chan deviceHealth),
		initialized:  false,
		lock:         &sync.Mutex{},
		starter: &DeviceStarter{
			started:  false,
			stopChan: make(chan struct{}),
			backoff:  defaultBackoffTime,
		},
		ctx: ctx,
	}
	return dpi
}

func constructVGPUDPIdevices(vGPUList []string) (devs []*pluginapi.Device) {
	for _, v := range vGPUList {
		dpiDev := &pluginapi.Device{
			ID:     v,
			Health: pluginapi.Unhealthy,
		}
		devs = append(devs, dpiDev)
	}
	return
}

// Set Started is used after a call to Start. It's purpose is to set the private starter properly
func (dp *VGPUDevicePlugin) SetStarted(stop chan struct{}) {
	c := dp.starter
	c.stopChan = stop
	c.started = true
	logrus.Infof("Started DevicePlugin: %s", dp.resourceName)
}

func (dp *VGPUDevicePlugin) Started() bool {
	return dp.starter.started
}

func (dp *VGPUDevicePlugin) Stop() error {
	return dp.stopDevicePlugin()
}

// Start starts the device plugin
func (dp *VGPUDevicePlugin) Start(stop <-chan struct{}) (err error) {
	dp.stop = stop
	dp.done = make(chan struct{})
	dp.deregistered = make(chan struct{})

	err = dp.cleanup()
	if err != nil {
		return err
	}

	sock, err := net.Listen("unix", dp.socketPath)
	if err != nil {
		return fmt.Errorf("error creating GRPC server socket: %v", err)
	}

	dp.server = grpc.NewServer([]grpc.ServerOption{}...)

	pluginapi.RegisterDevicePluginServer(dp.server, dp)

	errChan := make(chan error, 1)

	go func() {
		errChan <- dp.server.Serve(sock)
	}()

	err = waitForGRPCServer(dp.ctx, dp.socketPath, connectionTimeout)
	if err != nil {
		return fmt.Errorf("error starting the GRPC server: %v", err)
	}

	err = dp.register()
	if err != nil {
		return fmt.Errorf("error registering with device plugin manager: %v", err)
	}

	dp.setInitialized(true)
	logrus.Infof("Initialized DevicePlugin: %s", dp.resourceName)
	dp.starter.started = true
	err = <-errChan

	return err
}

func (dp *VGPUDevicePlugin) ListAndWatch(_ *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {

	errChan := make(chan error, 1)
	go func() {
		errChan <- dp.healthCheck()
	}()

	emptyList := []*pluginapi.Device{}
	err := s.Send(&pluginapi.ListAndWatchResponse{Devices: dp.devs})
	if err != nil {
		return err
	}
	done := false
	for {
		select {
		case devHealth := <-dp.health:
			for i, dev := range dp.devs {
				if devHealth.DevID == dev.ID {
					dp.devs[i].Health = devHealth.Health
				}
			}
			if err := s.Send(&pluginapi.ListAndWatchResponse{Devices: dp.devs}); err != nil {
				return err
			}
			logrus.Debugf("Sending ListAndWatchResponse for device with dpi.devs = %v", dp.devs)
		case <-dp.stop:
			done = true
		case <-dp.done:
			done = true
		}
		if done {
			break
		}
	}
	// Send empty list to increase the chance that the kubelet acts fast on stopped device plugins
	// There exists no explicit way to deregister devices
	if err := s.Send(&pluginapi.ListAndWatchResponse{Devices: emptyList}); err != nil {
		logrus.Errorf("%s device plugin failed to deregister: %s", dp.resourceName, err)
	}
	close(dp.deregistered)
	return <-errChan
}

func (dp *VGPUDevicePlugin) Allocate(_ context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	logrus.Debugf("Allocate request %s", r.String())
	resourceNameEnvVar := util.ResourceNameToEnvVar(vgpuPrefix, dp.resourceName)
	allocatedDevices := []string{}
	resp := new(pluginapi.AllocateResponse)
	containerResponse := new(pluginapi.ContainerAllocateResponse)

	for _, request := range r.ContainerRequests {
		deviceSpecs := make([]*pluginapi.DeviceSpec, 0)
		for _, devID := range request.DevicesIDs {
			logrus.Debugf("trying to allocate device for %s", devID)
			devicePath := filepath.Join(v1beta1.MdevRoot, devID)
			_, err := os.Stat(devicePath)
			if err != nil {
				logrus.Errorf("error allocating device %s: %v", devID, err)
				continue
			}
			deviceSpecs = append(deviceSpecs, &pluginapi.DeviceSpec{
				ContainerPath: vfioDevicePath,
				HostPath:      vfioDevicePath,
				Permissions:   "mrw",
			})
			allocatedDevices = append(allocatedDevices, devID)

		}
		containerResponse.Devices = deviceSpecs
		envVar := make(map[string]string)
		envVar[resourceNameEnvVar] = strings.Join(allocatedDevices, ",")

		containerResponse.Envs = envVar
		resp.ContainerResponses = append(resp.ContainerResponses, containerResponse)
		logrus.Debugf("Allocate response %v", resp)
	}

	return resp, nil
}

func (dp *VGPUDevicePlugin) healthCheck() error {
	monitoredDevices := make(map[string]string)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to creating a fsnotify watcher: %v", err)
	}
	defer watcher.Close()

	// This way we don't have to mount /dev from the node
	devicePath := filepath.Join(dp.deviceRoot, dp.devicePath)
	logrus.Infof("check device path %s", devicePath)
	// Start watching the files before we check for their existence to avoid races
	dirName := filepath.Dir(devicePath)
	err = watcher.Add(dirName)
	if err != nil {
		return fmt.Errorf("failed to add the device root path to the watcher: %v", err)
	}

	_, err = os.Stat(devicePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("could not stat the device: %v", err)
		}
	}

	// probe all devices
	for _, dev := range dp.devs {
		// get iommuGroup from PCI Addr
		vgpuDevice := filepath.Join(devicePath, dev.ID)
		logrus.Infof("adding device %s for watch in plugin %s", vgpuDevice, dp.resourceName)
		err = watcher.Add(vgpuDevice)
		if err != nil {
			return fmt.Errorf("failed to add the device %s to the watcher: %v", vgpuDevice, err)
		}
		monitoredDevices[dev.ID] = vgpuDevice
	}

	logrus.Infof("all monitored devices %s for plugin %s", monitoredDevices, dp.resourceName)
	dirName = filepath.Dir(dp.socketPath)
	err = watcher.Add(dirName)

	if err != nil {
		return fmt.Errorf("failed to add the device-plugin kubelet path to the watcher: %v", err)
	}
	_, err = os.Stat(dp.socketPath)
	if err != nil {
		return fmt.Errorf("failed to stat the device-plugin socket: %v", err)
	}

	err = watcher.Add(dp.socketPath)
	if err != nil {
		return fmt.Errorf("failed to watch device-plugin socket: %v", err)
	}

	return dp.performCheck(monitoredDevices, watcher)
}

// performCheck performs checks and monitors the devices
func (dp *VGPUDevicePlugin) performCheck(monitoredDevices map[string]string, watcher *fsnotify.Watcher) error {
	// run initial health check for devices created before. This works around device restarts
	// the device plugin runs out of band from the actual device enablement so the first device could be missed by the plugin
	for monDevID, devPath := range monitoredDevices {
		_, err := os.Stat(devPath)
		if err == nil {
			logrus.Infof("marking devID %s healthy for plugin %s", monDevID, dp.resourceName)
			dp.health <- deviceHealth{
				DevID:  monDevID,
				Health: pluginapi.Healthy,
			}
		}
	}

	for {
		select {
		case <-dp.stop:
			return nil
		case err := <-watcher.Errors:
			logrus.Errorf("error watching devices and device plugin directory: %v", err)
		case event := <-watcher.Events:
			logrus.Infof("got event for device %s in plugin %s", event.Name, dp.resourceName)
			if monDevID, exist := monitoredDevices[event.Name]; exist {
				// Health in this case is if the device path actually exists
				if event.Op == fsnotify.Create {
					logrus.Debugf("monitored device %s appeared", dp.resourceName)
					dp.health <- deviceHealth{
						DevID:  monDevID,
						Health: pluginapi.Healthy,
					}
				} else if (event.Op == fsnotify.Remove) || (event.Op == fsnotify.Rename) {
					logrus.Debugf("monitored device %s disappeared", dp.resourceName)
					dp.health <- deviceHealth{
						DevID:  monDevID,
						Health: pluginapi.Unhealthy,
					}
				}
			} else if event.Name == dp.socketPath && event.Op == fsnotify.Remove {
				logrus.Infof("device socket file for device %s was removed, kubelet probably restarted.", dp.resourceName)
				return nil
			}
		}
	}
}

func (dp *VGPUDevicePlugin) GetDeviceName() string {
	return dp.resourceName
}

// Stop stops the gRPC server
func (dp *VGPUDevicePlugin) stopDevicePlugin() error {

	if !IsChanClosed(dp.done) {
		close(dp.done)
	}

	// Give the device plugin 5 seconds to properly deregister
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	select {
	case <-dp.deregistered:
	case <-ticker.C:
	}

	dp.server.Stop()
	dp.setInitialized(false)
	return dp.cleanup()
}

// Register the device plugin for the given resourceName with Kubelet.
func (dp *VGPUDevicePlugin) register() error {
	conn, err := gRPCConnect(dp.ctx, pluginapi.KubeletSocket, connectionTimeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(dp.socketPath),
		ResourceName: dp.resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}
	return nil
}

func (dp *VGPUDevicePlugin) cleanup() error {
	if err := os.Remove(dp.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

func (dp *VGPUDevicePlugin) GetDevicePluginOptions(_ context.Context, _ *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	options := &pluginapi.DevicePluginOptions{
		PreStartRequired: false,
	}
	return options, nil
}

func (dp *VGPUDevicePlugin) PreStartContainer(_ context.Context, _ *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	res := &pluginapi.PreStartContainerResponse{}
	return res, nil
}

func (dp *VGPUDevicePlugin) setInitialized(initialized bool) {
	dp.lock.Lock()
	dp.initialized = initialized
	dp.lock.Unlock()
}

// This function adds the VGPU UUID to device plugin for corresponding VGPU type
func (dp *VGPUDevicePlugin) AddDevice(uuid string) error {
	var exists bool
	dp.lock.Lock()
	defer dp.lock.Unlock()
	for _, v := range dp.devs {
		if v.ID == uuid {
			exists = true
		}
	}
	if !exists {

		devs := constructVGPUDPIdevices([]string{uuid})
		dp.devs = append(dp.devs, devs...)
		dp.MarkVGPUDeviceAsHealthy(uuid)
	}

	return nil
}

// MarkVGPUDeviceAsHealthy marks the vGPU device as healthy
func (dp *VGPUDevicePlugin) MarkVGPUDeviceAsHealthy(uuid string) {
	go func() {
		dp.health <- deviceHealth{
			DevID:  uuid,
			Health: pluginapi.Healthy,
		}
	}()
}

// This function removes the VGPU ID from device plugin and also updates
// devs being reconilled by VGPUDevicePlugin
func (dp *VGPUDevicePlugin) RemoveDevice(uuid string) error {
	dp.lock.Lock()
	defer dp.lock.Unlock()
	if dp != nil {
		logrus.Infof("Removing %s from device plugin", uuid)
		dp.MarkVGPUDeviceAsUnHealthy(uuid)
	}

	for i, dev := range dp.devs {
		if dev.ID == uuid {
			dp.devs[i].Health = pluginapi.Unhealthy
		}
	}
	return nil
}

func (dp *VGPUDevicePlugin) MarkVGPUDeviceAsUnHealthy(uuid string) {
	dp.health <- deviceHealth{
		DevID:  uuid,
		Health: pluginapi.Unhealthy,
	}
}

func (dp *VGPUDevicePlugin) DeviceExists(uuid string) bool {
	for _, v := range dp.devs {
		if v.ID == uuid {
			return true
		}
	}
	return false
}
