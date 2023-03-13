package integration

import (
	"fmt"
	"time"

	devicesv1beta1 "github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("run node deletion tests", func() {

	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
		},
	}

	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2",
			DeletionTimestamp: &metav1.Time{
				Time: time.Now(),
			},
		},
	}

	claim1Node1 := &devicesv1beta1.PCIDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "dev1node1",
			Finalizers: []string{"wrangler.cattle.io/PCIDeviceClaimOnRemove"},
		},
		Spec: devicesv1beta1.PCIDeviceClaimSpec{
			NodeName: "node1",
		},
	}

	claim2Node1 := &devicesv1beta1.PCIDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "dev2node1",
			Finalizers: []string{"wrangler.cattle.io/PCIDeviceClaimOnRemove"},
		},
		Spec: devicesv1beta1.PCIDeviceClaimSpec{
			NodeName: "node1",
		},
	}

	claim1Node2 := &devicesv1beta1.PCIDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "dev1node2",
			Finalizers: []string{"wrangler.cattle.io/PCIDeviceClaimOnRemove"},
		},
		Spec: devicesv1beta1.PCIDeviceClaimSpec{
			NodeName: "node2",
		},
	}

	claim2Node2 := &devicesv1beta1.PCIDeviceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "dev2node2",
			Finalizers: []string{"wrangler.cattle.io/PCIDeviceClaimOnRemove"},
		},
		Spec: devicesv1beta1.PCIDeviceClaimSpec{
			NodeName: "node2",
		},
	}

	dev1Node1 := &devicesv1beta1.PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dev1node1",
			Labels: map[string]string{
				"nodename": "node1",
			},
		},
	}

	dev2Node1 := &devicesv1beta1.PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dev2node1",
			Labels: map[string]string{
				"nodename": "node1",
			},
		},
	}

	dev1Node2 := &devicesv1beta1.PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dev1node2",
			Labels: map[string]string{
				"nodename": "node2",
			},
		},
	}

	dev2Node2 := &devicesv1beta1.PCIDevice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dev2node2",
			Labels: map[string]string{
				"nodename": "node2",
			},
		},
	}
	BeforeEach(func() {
		Eventually(func() error {
			return k8sClient.Create(ctx, node1)
		}).ShouldNot(HaveOccurred())
		Eventually(func() error {
			return k8sClient.Create(ctx, node2)
		}).ShouldNot(HaveOccurred())
		Eventually(func() error {
			return k8sClient.Create(ctx, dev1Node1)
		}).ShouldNot(HaveOccurred())
		Eventually(func() error {
			return k8sClient.Create(ctx, dev2Node1)
		}).ShouldNot(HaveOccurred())
		Eventually(func() error {
			return k8sClient.Create(ctx, dev1Node2)
		}).ShouldNot(HaveOccurred())
		Eventually(func() error {
			return k8sClient.Create(ctx, dev2Node2)
		}).ShouldNot(HaveOccurred())
		Eventually(func() error {
			return k8sClient.Create(ctx, claim1Node1)
		}).ShouldNot(HaveOccurred())
		Eventually(func() error {
			return k8sClient.Create(ctx, claim2Node1)
		}).ShouldNot(HaveOccurred())
		Eventually(func() error {
			return k8sClient.Create(ctx, claim1Node2)
		}).ShouldNot(HaveOccurred())
		Eventually(func() error {
			return k8sClient.Create(ctx, claim2Node2)
		}).ShouldNot(HaveOccurred())
	})

	It("delete node2 and reconcile devices", func() {
		By("deleting node2", func() {
			Eventually(func() error {
				return k8sClient.Delete(ctx, node2)
			}).ShouldNot(HaveOccurred())
		})

		By("query node2", func() {
			Eventually(func() error {
				nodeObj := &corev1.Node{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: node2.Name, Namespace: node2.Namespace}, nodeObj)

				if apierrors.IsNotFound(err) {
					return nil
				}

				return err
			}).ShouldNot(HaveOccurred())
		})

		By("reconcile node2 pcidevices", func() {
			Eventually(func() error {
				devList := &devicesv1beta1.PCIDeviceList{}

				selector := labels.SelectorFromSet(map[string]string{
					"nodename": node2.Name,
				})

				if err := k8sClient.List(ctx, devList, &client.ListOptions{LabelSelector: selector}); err != nil {
					return err
				}

				if len(devList.Items) != 0 {
					return fmt.Errorf("expected to find no pcidevices for node2")
				}
				return nil
			}, "30s", "5s").ShouldNot(HaveOccurred())
		})

		By("reconcile node2 pcideviceclaims", func() {
			Eventually(func() error {
				claimList := &devicesv1beta1.PCIDeviceClaimList{}
				if err := k8sClient.List(ctx, claimList); err != nil {
					return err
				}

				GinkgoWriter.Println(claimList.Items)
				for _, v := range claimList.Items {
					if v.Spec.NodeName == node2.Name {
						return fmt.Errorf("found pcidevice claim for node2")
					}
				}
				return nil
			}, "30s", "5s").ShouldNot(HaveOccurred())
		})

	})

})
