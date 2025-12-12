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
	"kubevirt.io/client-go/log"
	"kubevirt.io/kubevirt/pkg/util"
	pluginapi "kubevirt.io/kubevirt/pkg/virt-handler/device-manager/deviceplugin/v1beta1"
)

const (
	vfioDevicePath    = "/dev/vfio/"
	vfioMount         = "/dev/vfio/vfio"
	pciBasePath       = "/sys/bus/pci/devices"
	connectionTimeout = 120 * time.Second // Google gRPC default timeout
	PCIResourcePrefix = "PCI_RESOURCE"
)

type PCIDevice struct {
	pciID      string
	driver     string
	pciAddress string
	iommuGroup string
	numaNode   int
}

type PCIDevicePlugin struct {
	pcidevs       []*PCIDevice
	devs          []*pluginapi.Device
	server        *grpc.Server
	socketPath    string
	health        chan deviceHealth
	devicePath    string
	resourceName  string
	done          chan struct{}
	deviceRoot    string
	iommuToPCIMap map[string]string
	initialized   bool
	lock          *sync.Mutex
	deregistered  chan struct{}
	starter       *DeviceStarter
	ctx           context.Context
}

type DeviceStarter struct {
	started  bool
	stopChan chan struct{}
	backoff  []time.Duration
}

func (dp *PCIDevicePlugin) GetPCIDevices() []*PCIDevice {
	return dp.pcidevs
}

// Not adding more data to the struct, it's big enough already
func (dp *PCIDevicePlugin) GetCount() int {
	var count int
	for _, dev := range dp.devs {
		if dev.Health == pluginapi.Healthy {
			count++
		}
	}
	return count
}

func (d *PCIDevice) GetID() string {
	return d.pciID
}

func NewPCIDevicePlugin(ctx context.Context, pciDevices []*PCIDevice, resourceName string) *PCIDevicePlugin {
	serverSock := SocketPath(strings.ReplaceAll(resourceName, "/", "-"))
	iommuToPCIMap := make(map[string]string)

	initHandler()

	devs := constructDPIdevices(pciDevices, iommuToPCIMap)
	dpi := &PCIDevicePlugin{
		pcidevs:       pciDevices,
		devs:          devs,
		socketPath:    serverSock,
		resourceName:  resourceName,
		devicePath:    vfioDevicePath,
		deviceRoot:    util.HostRootMount,
		iommuToPCIMap: iommuToPCIMap,
		health:        make(chan deviceHealth),
		initialized:   false,
		lock:          &sync.Mutex{},
		starter: &DeviceStarter{
			started:  false,
			stopChan: make(chan struct{}),
			backoff:  defaultBackoffTime,
		},
		ctx: ctx,
	}
	return dpi
}

func constructDPIdevices(pciDevices []*PCIDevice, iommuToPCIMap map[string]string) (devs []*pluginapi.Device) {
	for _, pciDevice := range pciDevices {
		iommuToPCIMap[pciDevice.pciAddress] = pciDevice.iommuGroup
		dpiDev := &pluginapi.Device{
			ID:     string(pciDevice.pciID),
			Health: pluginapi.Unhealthy,
		}
		if pciDevice.numaNode >= 0 {
			numaInfo := &pluginapi.NUMANode{
				ID: int64(pciDevice.numaNode),
			}
			dpiDev.Topology = &pluginapi.TopologyInfo{
				Nodes: []*pluginapi.NUMANode{numaInfo},
			}
		}
		devs = append(devs, dpiDev)
	}
	return
}

var defaultBackoffTime = []time.Duration{1 * time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second}

// Set Started is used after a call to Start. It's purpose is to set the private starter properly
func (dp *PCIDevicePlugin) SetStarted() {
	c := dp.starter
	c.started = true
	logrus.Infof("Started DevicePlugin: %s", dp.resourceName)
}

func (dp *PCIDevicePlugin) Started() bool {
	return dp.starter.started
}

func (dp *PCIDevicePlugin) Stop() error {
	return dp.stopDevicePlugin()
}

// Start starts the device plugin
func (dp *PCIDevicePlugin) Start() (err error) {
	logger := log.DefaultLogger()
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
	logger.Infof("Initialized DevicePlugin: %s", dp.resourceName)
	dp.starter.started = true
	err = <-errChan

	return err
}

func (dp *PCIDevicePlugin) ListAndWatch(_ *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {

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
			for _, dev := range dp.devs {
				if devHealth.DevID == dev.ID {
					dev.Health = devHealth.Health
				}
			}
			if err := s.Send(&pluginapi.ListAndWatchResponse{Devices: dp.devs}); err != nil {
				return err
			}
			logrus.Debugf("Sending ListAndWatchResponse for device with dpi.devs = %v", dp.devs)
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
		log.DefaultLogger().Reason(err).Infof("%s device plugin failed to deregister: %s", dp.resourceName, err)
	}
	close(dp.deregistered)
	return <-errChan
}

func (dp *PCIDevicePlugin) Allocate(_ context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	logrus.Debugf("Allocate request %s", r.String())
	resourceNameEnvVar := util.ResourceNameToEnvVar(PCIResourcePrefix, dp.resourceName)
	allocatedDevices := []string{}
	resp := new(pluginapi.AllocateResponse)
	containerResponse := new(pluginapi.ContainerAllocateResponse)

	for _, request := range r.ContainerRequests {
		deviceSpecs := make([]*pluginapi.DeviceSpec, 0)
		for _, devID := range request.DevicesIDs {
			// translate device's iommu group to its pci address
			logrus.Debugf("looking up deviceID %s in map %v", devID, dp.iommuToPCIMap)
			iommuGroup, exist := dp.iommuToPCIMap[devID] // not finding device ids
			if exist {
				// if device exists, check if there other devices
				// in the same iommuGroup, and append these too
				allocatedDevices = append(allocatedDevices, devID)
				for devPCIAddress, ig := range dp.iommuToPCIMap {
					if ig == iommuGroup {
						allocatedDevices = append(allocatedDevices, devPCIAddress)
					}
				}
				deviceSpecs = append(deviceSpecs, formatVFIODeviceSpecs(iommuGroup)...)
			} else {
				continue // break execution of loop as we are not handling this device
			}
		}
		containerResponse.Devices = deviceSpecs
		envVar := make(map[string]string)
		envVar[resourceNameEnvVar] = strings.Join(allocatedDevices, ",")

		containerResponse.Envs = envVar
		resp.ContainerResponses = append(resp.ContainerResponses, containerResponse)
	}
	logrus.Debugf("Allocate response %v", resp)
	return resp, nil
}

func (dp *PCIDevicePlugin) healthCheck() error {
	logger := log.DefaultLogger()
	monitoredDevices := make(map[string]string)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to creating a fsnotify watcher: %v", err)
	}
	defer watcher.Close()

	// This way we don't have to mount /dev from the node
	devicePath := filepath.Join(dp.deviceRoot, dp.devicePath)

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
		for pciAddr, iommuGroup := range dp.iommuToPCIMap {
			if pciAddr == dev.ID {
				vfioDevice := filepath.Join(devicePath, iommuGroup)
				err = watcher.Add(vfioDevice)
				if err != nil {
					return fmt.Errorf("failed to add the device %s to the watcher: %v", vfioDevice, err)
				}
				monitoredDevices[dev.ID] = vfioDevice
			}
		}
	}

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

	for {
		select {
		case err := <-watcher.Errors:
			logger.Reason(err).Errorf("error watching devices and device plugin directory")
		case event := <-watcher.Events:
			logger.V(4).Infof("health Event: %v", event)
			if monDevID, exist := monitoredDevices[event.Name]; exist {
				// Health in this case is if the device path actually exists
				if event.Op == fsnotify.Create {
					logger.Infof("monitored device %s appeared", dp.resourceName)
					dp.health <- deviceHealth{
						DevID:  monDevID,
						Health: pluginapi.Healthy,
					}
				} else if (event.Op == fsnotify.Remove) || (event.Op == fsnotify.Rename) {
					logger.Infof("monitored device %s disappeared", dp.resourceName)
					dp.health <- deviceHealth{
						DevID:  monDevID,
						Health: pluginapi.Unhealthy,
					}
				}
			} else if event.Name == dp.socketPath && event.Op == fsnotify.Remove {
				logger.Infof("device socket file for device %s was removed, kubelet probably restarted.", dp.resourceName)
				return nil
			}
		}
	}
}

func (dp *PCIDevicePlugin) GetDevicePath() string {
	return dp.devicePath
}

func (dp *PCIDevicePlugin) GetDeviceName() string {
	return dp.resourceName
}

// Stop stops the gRPC server
func (dp *PCIDevicePlugin) stopDevicePlugin() error {
	defer func() {
		if !IsChanClosed(dp.done) {
			close(dp.done)
		}
	}()

	// Give the device plugin one second to properly deregister
	ticker := time.NewTicker(1 * time.Second)
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
func (dp *PCIDevicePlugin) register() error {
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

func (dp *PCIDevicePlugin) cleanup() error {
	if err := os.Remove(dp.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

func (dp *PCIDevicePlugin) GetDevicePluginOptions(_ context.Context, _ *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	options := &pluginapi.DevicePluginOptions{
		PreStartRequired: false,
	}
	return options, nil
}

func (dp *PCIDevicePlugin) PreStartContainer(_ context.Context, _ *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	res := &pluginapi.PreStartContainerResponse{}
	return res, nil
}

func (dp *PCIDevicePlugin) GetInitialized() bool {
	dp.lock.Lock()
	defer dp.lock.Unlock()
	return dp.initialized
}

func (dp *PCIDevicePlugin) setInitialized(initialized bool) {
	dp.lock.Lock()
	dp.initialized = initialized
	dp.lock.Unlock()
}
