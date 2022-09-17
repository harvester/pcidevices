package integration

import (
	"fmt"

	"github.com/harvester/pcidevices/pkg/webhook"
	"github.com/harvester/pcidevices/tests/helpers"
	corev1 "k8s.io/api/core/v1"

	"github.com/google/uuid"
	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("validate mutator by sending a mock pod request needing mutation", func() {

	vmName := "testVM"

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
		})
	})
})

var _ = Describe("validate mutator by sending a mock pod request not needing mutation", func() {

	vmName := "testVM"

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
		})
	})
})
