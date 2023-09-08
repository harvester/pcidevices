package pcideviceclaim

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/u-root/u-root/pkg/kmodule"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/deviceplugins"
	v1beta1gen "github.com/harvester/pcidevices/pkg/generated/controllers/devices.harvesterhci.io/v1beta1"
)

var (
	// the pcidevice-controller runs 2 threads for all the registered handlers
	// as a result there may be cases when processing multiple pcideviceclaims
	// for same device types, there is a race condition in creating/updating the
	// deviceplugin to register additional pcidevice addresses.
	// the lock has been introduced to ensure serial updates to the deviceplugin map
	// stored in the handler.
	// the lock is called during pcideviceclaim creation and deletion operations
	// to ensure that the deviceplugin updates are performed serially.
	// this in turn helps ensure that the correct device capacity / allocation
	// is updated in the corev1.NodeStatus.Allocatable and corev1.NodeStatus.Capacity
	lock sync.Mutex
)

const (
	vfioPCIDriver     = "vfio-pci"
	DefaultNS         = "harvester-system"
	KubevirtCR        = "kubevirt"
	vfioPCIDriverPath = "/sys/bus/pci/drivers/vfio-pci"
)

type Controller struct {
	PCIDeviceClaims v1beta1gen.PCIDeviceClaimController
}

type Handler struct {
	ctx           context.Context
	pdcClient     v1beta1gen.PCIDeviceClaimController
	pdClient      v1beta1gen.PCIDeviceClient
	virtClient    kubecli.KubevirtClient
	nodeName      string
	devicePlugins map[string]*deviceplugins.PCIDevicePlugin
}

func Register(
	ctx context.Context,
	pdcClient v1beta1gen.PCIDeviceClaimController,
	pdClient v1beta1gen.PCIDeviceController,
) error {
	logrus.Info("Registering PCI Device Claims controller")
	nodeName := os.Getenv(v1beta1.NodeEnvVarName)
	clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
	if err != nil {
		msg := fmt.Sprintf("cannot obtain KubeVirt client: %v", err)
		return errors.New(msg)
	}

	handler := &Handler{
		ctx:           ctx,
		pdcClient:     pdcClient,
		pdClient:      pdClient,
		nodeName:      nodeName,
		virtClient:    virtClient,
		devicePlugins: make(map[string]*deviceplugins.PCIDevicePlugin),
	}

	pdcClient.OnRemove(ctx, "PCIDeviceClaimOnRemove", handler.OnRemove)
	pdcClient.OnChange(ctx, "PCIDeviceClaimReconcile", handler.reconcilePCIDeviceClaims)
	// Watch to check for updates to pcidevices. This can happen on reboot as devices are set to reflect the correct
	// driver in use by said device. This helps ensure that associated claim is reconciled to trigger rebind if needed
	relatedresource.WatchClusterScoped(ctx, "PCIDeviceToClaimReconcile", handler.OnDeviceChange, pdcClient, pdClient)
	err = handler.unbindOrphanedPCIDevices()
	if err != nil {
		return err
	}
	// Load VFIO drivers when controller starts instead of repeatedly in the reconcile loop
	loadVfioDrivers()
	return nil
}

// When a PCIDeviceClaim is removed, we need to unbind the device from the vfio-pci driver
func (h *Handler) OnRemove(_ string, pdc *v1beta1.PCIDeviceClaim) (*v1beta1.PCIDeviceClaim, error) {
	if pdc == nil || pdc.DeletionTimestamp == nil {
		return pdc, nil
	}

	// need to requeue object to ensure correct node picks up and cleanup/rebind is executed
	if pdc.Spec.NodeName != h.nodeName {
		return pdc, fmt.Errorf("controller %s cannot process claims for node %s", h.nodeName, pdc.Spec.NodeName)
	}

	// Get PCIDevice for the PCIDeviceClaim
	pd, err := h.getPCIDeviceForClaim(pdc)
	if err != nil {
		logrus.Errorf("Error getting claim's device: %s", err)
		return pdc, err
	}

	lock.Lock()
	defer lock.Unlock()

	// Disable PCI Passthrough by unbinding from the vfio-pci device driver
	err = h.disablePassthrough(pd)
	if err != nil {
		return pdc, err
	}

	// Find the DevicePlugin
	resourceName := pd.Status.ResourceName
	dp := deviceplugins.Find(
		resourceName,
		h.devicePlugins,
	)

	if dp != nil {
		err = dp.RemoveDevice(pd, pdc)
		if err != nil {
			return pdc, err
		}
		// Check if that was the last device, and then shut down the dp
		time.Sleep(5 * time.Second)
		if dp.GetCount() == 0 {
			err := dp.Stop()
			if err != nil {
				return pdc, err
			}
			delete(h.devicePlugins, resourceName)
		}
	}
	return pdc, nil
}

func loadVfioDrivers() {
	for _, driver := range []string{"vfio-pci", "vfio_iommu_type1"} {
		logrus.Infof("Loading driver %s", driver)
		if err := kmodule.Probe(driver, ""); err != nil {
			logrus.Error(err)
		}
	}
}

func bindDeviceToVFIOPCIDriver(pd *v1beta1.PCIDevice) error {
	if deviceBoundToDriver(vfioPCIDriverPath, pd.Status.Address) {
		return nil
	}

	vendorID := pd.Status.VendorID
	deviceID := pd.Status.DeviceID
	id := fmt.Sprintf("%s %s", vendorID, deviceID)
	logrus.Infof("Binding device %s [%s] to vfio-pci", pd.Name, id)

	newIDFile, err := os.OpenFile("/sys/bus/pci/drivers/vfio-pci/new_id", os.O_WRONLY, 0200)
	if err != nil {
		return fmt.Errorf("error opening new_id file: %v", err)
	}
	defer newIDFile.Close()
	_, err = newIDFile.WriteString(id)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("error writing to new_id file: %v", err)
	}

	logrus.Infof("Binding device %s vfio-pci", pd.Status.Address)
	file, err := os.OpenFile("/sys/bus/pci/drivers/vfio-pci/bind", os.O_WRONLY, 0200)
	if err != nil {
		return fmt.Errorf("error opening bind file: %s", err)
	}
	_, err = file.WriteString(pd.Status.Address)
	if err != nil {
		file.Close()
		return fmt.Errorf("error writing to bind file: %s", err)
	}
	file.Close()

	if !deviceBoundToDriver(vfioPCIDriverPath, pd.Status.Address) {
		return fmt.Errorf("no device %s found at /sys/bus/pci/drivers/vfio-pci", pd.Status.Address)
	}
	return nil
}

// Enabling passthrough for a PCI Device requires two steps:
// 1. Bind the device to the vfio-pci driver in the host
// 2. Add device to DevicePlugin so KubeVirt will recognize it
func (h Handler) enablePassthrough(pd *v1beta1.PCIDevice) error {
	err := bindDeviceToVFIOPCIDriver(pd)
	if err != nil {
		return err
	}
	pdCopy := pd.DeepCopy()
	pdCopy.Status.KernelDriverInUse = vfioPCIDriver
	_, err = h.pdClient.UpdateStatus(pdCopy)
	return err
}

// disablePassthrough will unbind and bind device to the original driver
func (h *Handler) disablePassthrough(pd *v1beta1.PCIDevice) error {
	err := unbindDeviceFromDriver(pd.Status.Address, vfioPCIDriver)
	if err != nil {
		return fmt.Errorf("failed unbinding driver: (%s)", err)
	}

	return h.bindDeviceToOriginalDriver(pd)
}

// This function unbinds the device with PCI Address addr from the given driver
// NOTE: this function assumes that addr is on THIS NODE, only call for PCI addrs on this node
func unbindDeviceFromDriver(addr string, driver string) error {
	driverPath := fmt.Sprintf("/sys/bus/pci/drivers/%s", driver)
	// Check if device at addr is already bound to driver
	if !deviceBoundToDriver(driverPath, addr) {
		return nil
	}
	path := fmt.Sprintf("%s/unbind", driverPath)
	file, err := os.OpenFile(path, os.O_WRONLY, 0200)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(addr)
	if err != nil {
		return err
	}

	if deviceBoundToDriver(driverPath, addr) {
		return fmt.Errorf("device still bound to driver, will check again")
	}
	return nil
}

func pciDeviceIsClaimed(pd *v1beta1.PCIDevice, pdcs *v1beta1.PCIDeviceClaimList, nodeName string) bool {
	for _, pdc := range pdcs.Items {
		if pd.Status.NodeName != nodeName {
			continue
		}
		if pdc.OwnerReferences == nil {
			return false
		}
		if pdc.OwnerReferences[0].Name == pd.Name {
			return true
		}
	}
	return false
}

// A PCI Device is considered orphaned if it is bound to vfio-pci,
// but has no PCIDeviceClaim. The assumption is that this controller
// will manage all PCI passthrough, and consider orphaned devices invalid
func getOrphanedPCIDevices(
	pdcs *v1beta1.PCIDeviceClaimList,
	pds *v1beta1.PCIDeviceList, nodeName string) (*v1beta1.PCIDeviceList, error) {
	pdsOrphaned := v1beta1.PCIDeviceList{}
	for i := range pds.Items {
		pd := pds.Items[i] // fix G601: Implicit memory aliasing in for loop. (gosec)
		isVfioPci := pd.Status.KernelDriverInUse == "vfio-pci"
		isOnThisNode := nodeName == pd.Status.NodeName
		if isVfioPci && isOnThisNode && !pciDeviceIsClaimed(&pd, pdcs, nodeName) {
			pdsOrphaned.Items = append(pdsOrphaned.Items, *pd.DeepCopy())
		}
	}
	return &pdsOrphaned, nil
}

func (h *Handler) reconcilePCIDeviceClaims(_ string, pdc *v1beta1.PCIDeviceClaim) (*v1beta1.PCIDeviceClaim, error) {

	if pdc == nil || pdc.DeletionTimestamp != nil || (pdc.Spec.NodeName != h.nodeName) {
		return pdc, nil
	}

	pdcCopy := pdc.DeepCopy()
	// Get the PCIDevice object for the PCIDeviceClaim
	pd, err := h.getPCIDeviceForClaim(pdc)
	if pd == nil {
		return pdc, err
	}

	lock.Lock()
	defer lock.Unlock()

	if err := h.permitHostDeviceInKubeVirt(pd); err != nil {
		return pdc, fmt.Errorf("error updating kubevirt CR: %v", err)
	}

	// Enable PCI Passthrough on the device by binding it to vfio-pci driver
	err = h.attemptToEnablePassthrough(pd, pdc)
	if err != nil {
		return pdc, err
	}

	// Find the DevicePlugin
	resourceName := pd.Status.ResourceName
	dp := deviceplugins.Find(
		resourceName,
		h.devicePlugins,
	)
	if dp == nil {
		pds := []*v1beta1.PCIDevice{pd}
		dp, err = h.createDevicePlugin(pds, pdc)
		if err != nil {
			return pdc, err
		}
		// new plugin created. Need to store state.
		h.devicePlugins[resourceName] = dp
	} else {
		// Add the Device to the DevicePlugin
		if err := dp.AddDevice(pd, pdc); err != nil {
			return pdc, err
		}
	}

	if !pdcCopy.Status.PassthroughEnabled {
		pdcCopy.Status.PassthroughEnabled = true
		pdcCopy.Status.KernelDriverToUnbind = pd.Status.KernelDriverInUse
		return h.pdcClient.UpdateStatus(pdcCopy)
	}

	return pdc, nil
}

func (h *Handler) createDevicePlugin(
	pds []*v1beta1.PCIDevice,
	pdc *v1beta1.PCIDeviceClaim,
) (*deviceplugins.PCIDevicePlugin, error) {
	resourceName := pds[0].Status.ResourceName
	logrus.Infof("Creating DevicePlugin: %s", resourceName)
	dp := deviceplugins.Create(h.ctx, resourceName, pdc.Spec.Address, pds)
	h.devicePlugins[resourceName] = dp
	// Start the DevicePlugin
	if pdc.Status.PassthroughEnabled && !dp.Started() {
		err := h.startDevicePlugin(dp)
		if err != nil {
			return nil, err
		}
	}
	return dp, nil
}

func (h *Handler) permitHostDeviceInKubeVirt(pd *v1beta1.PCIDevice) error {
	logrus.Infof("Adding %s to KubeVirt list of permitted devices", pd.Name)
	kv, err := h.virtClient.KubeVirt(DefaultNS).Get(KubevirtCR, &v1.GetOptions{})
	if err != nil {
		msg := fmt.Sprintf("cannot obtain KubeVirt CR: %v", err)
		return errors.New(msg)
	}

	kvCopy := reconcileKubevirtCR(kv, pd)
	if !reflect.DeepEqual(kv.Spec.Configuration.PermittedHostDevices, kvCopy.Spec.Configuration.PermittedHostDevices) {
		_, err := h.virtClient.KubeVirt(DefaultNS).Update(kvCopy)
		return err
	}

	return nil
}

func reconcileKubevirtCR(kvObj *kubevirtv1.KubeVirt, pd *v1beta1.PCIDevice) *kubevirtv1.KubeVirt {
	kv := kvObj.DeepCopy()
	if kv.Spec.Configuration.PermittedHostDevices == nil {
		kv.Spec.Configuration.PermittedHostDevices = &kubevirtv1.PermittedHostDevices{
			PciHostDevices: []kubevirtv1.PciHostDevice{},
		}
	}
	permittedPCIDevices := kv.Spec.Configuration.PermittedHostDevices.PciHostDevices
	resourceName := pd.Status.ResourceName
	// check if device is currently permitted
	devPermitted := false
	for i, permittedPCIDev := range permittedPCIDevices {
		if permittedPCIDev.ResourceName == resourceName {
			if permittedPCIDev.ExternalResourceProvider {
				devPermitted = true
			}
			// remove device so it can be re-added
			permittedPCIDevices = append(permittedPCIDevices[:i], permittedPCIDevices[i+1:]...)
			break
		}
	}

	if !devPermitted {
		vendorID := pd.Status.VendorID
		deviceID := pd.Status.DeviceID
		devToPermit := kubevirtv1.PciHostDevice{
			PCIVendorSelector:        fmt.Sprintf("%s:%s", vendorID, deviceID),
			ResourceName:             resourceName,
			ExternalResourceProvider: true,
		}
		kv.Spec.Configuration.PermittedHostDevices.PciHostDevices = append(permittedPCIDevices, devToPermit)
	}
	return kv
}

func (h *Handler) getPCIDeviceForClaim(pdc *v1beta1.PCIDeviceClaim) (*v1beta1.PCIDevice, error) {
	// Get PCIDevice for the PCIDeviceClaim
	if pdc.OwnerReferences == nil {
		msg := fmt.Sprintf("Cannot find PCIDevice that owns %s", pdc.Name)
		return nil, errors.New(msg)
	}
	name := pdc.OwnerReferences[0].Name
	pd, err := h.pdClient.Get(name, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("error getting claim's device: %s", err)
		return nil, err
	}
	return pd, nil
}

func (h *Handler) startDevicePlugin(
	dp *deviceplugins.PCIDevicePlugin,
) error {
	if dp.Started() {
		return nil
	}
	// Start the plugin
	stop := make(chan struct{})
	go func() {
		err := dp.Start(stop)
		if err != nil {
			logrus.Errorf("error starting %s device plugin: %s", dp.GetDeviceName(), err)
		}
		// TODO: test if deleting this stops the DevicePlugin
		<-stop
	}()
	dp.SetStarted(stop)
	return nil
}

func (h *Handler) attemptToEnablePassthrough(pd *v1beta1.PCIDevice, pdc *v1beta1.PCIDeviceClaim) error {
	if !deviceBoundToDriver(vfioPCIDriverPath, pd.Status.Address) {
		logrus.Infof("Enabling passthrough for PDC: %s", pdc.Name)
		// Only unbind from driver is a driver is currently in use
		if strings.TrimSpace(pd.Status.KernelDriverInUse) != "" {
			err := unbindDeviceFromDriver(pd.Status.Address, pd.Status.KernelDriverInUse)
			if err != nil {
				return err
			}
		}

		originalDriver, ok := pd.Annotations[v1beta1.PciDeviceDriver]
		if ok {
			err := unbindDeviceFromDriver(pd.Status.Address, originalDriver)
			if err != nil {
				return err
			}
		}
		// Enable PCI Passthrough by binding the device to the vfio-pci driver
		err := h.enablePassthrough(pd)
		if err != nil {
			return err
		}
	}

	pdc.Status.PassthroughEnabled = true
	return nil

}

func (h *Handler) unbindOrphanedPCIDevices() error {
	pdcs, err := h.pdcClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	pds, err := h.pdClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	orphanedPCIDevices, err := getOrphanedPCIDevices(pdcs, pds, h.nodeName)
	if err != nil {
		return err
	}
	for _, pd := range orphanedPCIDevices.Items {
		if err := unbindDeviceFromDriver(pd.Status.Address, vfioPCIDriver); err != nil {
			return err
		}
	}
	return nil
}

// OnDeviceChange will watch the PCIDevice objects and trigger a reconcile of related PCIDeviceClaims if the underlying PCIDevice objects has a change.
// this can happen at reboot when device driver is updated to reflect in use device driver
func (h *Handler) OnDeviceChange(_ string, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if pd, ok := obj.(*v1beta1.PCIDevice); ok {
		if pd.Status.NodeName == h.nodeName && pd.Status.KernelDriverInUse != vfioPCIDriver {
			pdcList, err := h.pdcClient.List(metav1.ListOptions{LabelSelector: fmt.Sprintf("nodename=%s", h.nodeName)})
			if err != nil {
				return nil, fmt.Errorf("error listing PCIDeviceClaims during device watch: %v", err)
			}
			var rr []relatedresource.Key
			for _, v := range pdcList.Items {
				for _, owner := range v.GetOwnerReferences() {
					if owner.Kind == pd.Kind && owner.APIVersion == pd.APIVersion && owner.Name == pd.Name {
						rr = append(rr, relatedresource.NewKey(v.Namespace, v.Name))
					}
				}
			}
			return rr, nil
		}
	}

	return nil, nil
}

func (h *Handler) bindDeviceToOriginalDriver(pd *v1beta1.PCIDevice) error {
	address := pd.Status.Address
	orgDriver, ok := pd.Annotations[v1beta1.PciDeviceDriver]

	if !ok {
		logrus.Infof("no annotation %s found for original device driver on pcidevice %s, ignoring bind to original driver", v1beta1.PciDeviceDriver, pd.Name)
		return nil
	}

	if orgDriver == "" {
		logrus.Debugf("no original driver present on pcidevice: %s", pd.Name)
		return nil
	}

	logrus.Debugf("Binding device %s [%s] to %s", pd.Name, address, orgDriver)
	file, err := os.OpenFile(fmt.Sprintf("/sys/bus/pci/drivers/%s/bind", orgDriver), os.O_WRONLY, 0200)
	if err != nil {
		logrus.Errorf("Error opening bind file: %s", err)
		return err
	}
	_, err = file.WriteString(address)
	if err != nil {
		logrus.Errorf("Error writing to bind file: %s", err)
		file.Close()
		return err
	}
	file.Close()
	pdCopy := pd.DeepCopy()

	// update to reflect the original driver
	pdCopy.Status.KernelDriverInUse = orgDriver
	_, err = h.pdClient.UpdateStatus(pdCopy)
	return err
}

func deviceBoundToDriver(driverPath string, pciAddress string) bool {
	_, err := os.Stat(fmt.Sprintf("%s/%s", driverPath, pciAddress))
	return err == nil
}
