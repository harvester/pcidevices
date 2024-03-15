package integration

import (
	"fmt"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/webhook"
	"github.com/harvester/pcidevices/tests/helpers"
)

var _ = Describe("validate mutator by sending a mock pod request needing mutation", func() {

	vmName := "test-vm"

	claim := &v1beta1.PCIDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "int-test-claim",
			Namespace: "default",
		},
		Spec: v1beta1.PCIDeviceClaimSpec{
			Address:  "000-0000",
			NodeName: "localhost",
			UserName: "root",
		},
	}

	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmName,
			Namespace: "default",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							HostDevices: []kubevirtv1.HostDevice{
								{
									Name:       "dev1",
									DeviceName: "pcidevice-0000",
								},
							},
						},
					},
				},
			},
		},
	}

	device := &v1beta1.PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node1-dev1",
			Namespace: "default",
		},
		Spec: v1beta1.PCIDeviceSpec{},
		Status: v1beta1.PCIDeviceStatus{
			Address:      "00000",
			VendorID:     "ab",
			DeviceID:     "1f3c",
			ClassID:      "0C05",
			NodeName:     "node1",
			ResourceName: "pcidevice-0000",
			Description:  "fake device",
		},
	}

	deviceStatus := v1beta1.PCIDeviceStatus{
		Address:      "00000",
		VendorID:     "ab",
		DeviceID:     "1f3c",
		ClassID:      "0C05",
		NodeName:     "node1",
		ResourceName: "pcidevice-0000",
		Description:  "fake device",
	}

	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "virt-launcher-fake",
			Namespace: "default",
			Labels: map[string]string{
				webhook.VMLabel: vmName,
				"kubevirt.io":   "virt-launcher",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "compute",
					Image: "fakeimage",
					SecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"NET_BIND_SERVICE",
								"SYS_NICE",
							},
							Drop: []corev1.Capability{
								"NET_RAW",
							},
						},
					},
				},
			},
		},
	}
	BeforeEach(func() {
		// create a pci device claim
		Eventually(func() error {
			return k8sClient.Create(ctx, claim)
		}).ShouldNot(HaveOccurred())
		// create a pcidevice
		Eventually(func() error {
			return k8sClient.Create(ctx, device)
		}).ShouldNot(HaveOccurred())
		// create a vm
		Eventually(func() error {
			return k8sClient.Create(ctx, vm)
		}).ShouldNot(HaveOccurred())
	})

	It("run pcideviceclaim mutation tests", func() {
		By("fetch pcidevice", func() {
			Eventually(func() error {
				d := &v1beta1.PCIDevice{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: device.Name, Namespace: device.Namespace}, d)
				if err != nil {
					return err
				}
				GinkgoWriter.Println(d.Status)
				d.Status = deviceStatus
				return k8sClient.Status().Update(ctx, d)
			}).ShouldNot(HaveOccurred())
		})

		By("simulate webhook call via a fake pod", func() {
			Eventually(func() error {
				resp, err := helpers.GenerateMutationRequest(p, "https://localhost:8443/v1/webhook/mutation", cfg)
				if err != nil {
					return fmt.Errorf("error during mock mutation call: %v", err)
				}

				if resp == nil {
					return fmt.Errorf("expected to find a response from mutating webhook")
				}

				if len(resp.Patch) == 0 {
					return fmt.Errorf("expected to find a patch, but got nothing")
				}

				if !resp.Allowed {
					return fmt.Errorf("expected to request to be allowed, but got not allowed")
				}
				return nil
			}, "30s", "5s").ShouldNot(HaveOccurred())
		})

	})

	AfterEach(func() {
		Eventually(func() error {
			return k8sClient.Delete(ctx, claim)
		}).ShouldNot(HaveOccurred())
		// create a pcidevice
		Eventually(func() error {
			return k8sClient.Delete(ctx, device)
		}).ShouldNot(HaveOccurred())
		// create a vm
		Eventually(func() error {
			return k8sClient.Delete(ctx, vm)
		}).ShouldNot(HaveOccurred())

	})
})

var _ = Describe("validate mutator by sending a mock pod request not needing mutation", func() {

	vmName := "test-vm"

	claim := &v1beta1.PCIDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "int-nomod-claim",
			Namespace: "default",
		},
		Spec: v1beta1.PCIDeviceClaimSpec{
			Address:  "000-0000",
			NodeName: "localhost",
			UserName: "root",
		},
	}

	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmName,
			Namespace: "default",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							HostDevices: []kubevirtv1.HostDevice{
								{
									Name:       "dev1",
									DeviceName: "pcidevice-0000",
								},
							},
						},
					},
				},
			},
		},
	}

	device := &v1beta1.PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node1-dev1",
			Namespace: "default",
		},
		Spec: v1beta1.PCIDeviceSpec{},
		Status: v1beta1.PCIDeviceStatus{
			Address:      "00000",
			VendorID:     "ab",
			DeviceID:     "1f3c",
			ClassID:      "0C05",
			NodeName:     "node1",
			ResourceName: "pcidevice-0000",
			Description:  "fake device",
		},
	}

	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "virt-launcher-fake",
			Namespace: "default",
			Labels: map[string]string{
				webhook.VMLabel: vmName,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "fakepod",
					Image: "fakeimage",
					SecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"NET_BIND_SERVICE",
								"SYS_NICE",
							},
							Drop: []corev1.Capability{
								"NET_RAW",
							},
						},
					},
				},
			},
		},
	}
	BeforeEach(func() {
		// create a pci device claim
		Eventually(func() error {
			return k8sClient.Create(ctx, claim)
		}).ShouldNot(HaveOccurred())
		// create a pcidevice
		Eventually(func() error {
			return k8sClient.Create(ctx, device)
		}).ShouldNot(HaveOccurred())
		// create a vm
		Eventually(func() error {
			return k8sClient.Create(ctx, vm)
		}).ShouldNot(HaveOccurred())
	})

	It("run pcideviceclaim mutation tests", func() {
		By("set owner on pcideviceclaim to mimic a VM creation", func() {
			Eventually(func() error {
				claimObj := &v1beta1.PCIDeviceClaim{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: claim.Name, Namespace: claim.Namespace}, claimObj); err != nil {
					return fmt.Errorf("error fetching claim obj: %v", err)
				}

				owners := []metav1.OwnerReference{
					{
						Name:       vmName,
						Kind:       "VirtualMachine",
						APIVersion: "kubevirt.io/v1",
						UID:        types.UID(uuid.New().String()),
					},
				}
				claimObj.SetOwnerReferences(owners)
				return k8sClient.Update(ctx, claimObj)
			}, "30s", "5s").ShouldNot(HaveOccurred())

		})

		By("simulate webhook call via a fake pod", func() {
			Eventually(func() error {
				resp, err := helpers.GenerateMutationRequest(p, "https://localhost:8443/v1/webhook/mutation", cfg)
				if err != nil {
					return fmt.Errorf("error during mock mutation call: %v", err)
				}

				if resp == nil {
					return fmt.Errorf("expected to find a response from mutating webhook")
				}

				if len(resp.Patch) != 0 {
					return fmt.Errorf("expected to not find a patch")
				}

				if !resp.Allowed {
					return fmt.Errorf("expected to request to be allowed, but got not allowed")
				}
				return nil
			}, "30s", "5s").ShouldNot(HaveOccurred())
		})

	})

	AfterEach(func() {
		Eventually(func() error {
			return k8sClient.Delete(ctx, claim)
		}).ShouldNot(HaveOccurred())
		// create a pcidevice
		Eventually(func() error {
			return k8sClient.Delete(ctx, device)
		}).ShouldNot(HaveOccurred())
		// create a vm
		Eventually(func() error {
			return k8sClient.Delete(ctx, vm)
		}).ShouldNot(HaveOccurred())
	})
})
