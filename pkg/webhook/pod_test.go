package webhook

import (
	"context"
	"fmt"
	"strings"
	"testing"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/rancher/wrangler/pkg/yaml"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	DefaultPod = `
apiVersion: v1
kind: Pod
metadata:
  annotations:
    cni.projectcalico.org/containerID: d47db3f2f8a0a3a51bbaf3c6cf16e8ae7d57f29b16b27b108ff3477b9dd8f20e
    cni.projectcalico.org/podIP: 10.52.1.173/32
    cni.projectcalico.org/podIPs: 10.52.1.173/32
    harvesterhci.io/sshNames: '["default/demo"]'
    k8s.v1.cni.cncf.io/network-status: |-
      [{
          "name": "",
          "ips": [
              "10.52.1.173"
          ],
          "default": true,
          "dns": {}
      },{
          "name": "default/fremont",
          "interface": "net1",
          "mac": "aa:ef:94:6d:2f:f7",
          "dns": {}
      }]
    k8s.v1.cni.cncf.io/networks: '[{"interface":"net1","mac":"aa:ef:94:6d:2f:f7","name":"fremont","namespace":"default"}]'
    k8s.v1.cni.cncf.io/networks-status: |-
      [{
          "name": "",
          "ips": [
              "10.52.1.173"
          ],
          "default": true,
          "dns": {}
      },{
          "name": "default/fremont",
          "interface": "net1",
          "mac": "aa:ef:94:6d:2f:f7",
          "dns": {}
      }]
    kubernetes.io/psp: global-unrestricted-psp
    kubevirt.io/domain: demo
    kubevirt.io/migrationTransportUnix: "true"
    post.hook.backup.velero.io/command: '["/usr/bin/virt-freezer", "--unfreeze", "--name",
      "demo", "--namespace", "default"]'
    post.hook.backup.velero.io/container: compute
    pre.hook.backup.velero.io/command: '["/usr/bin/virt-freezer", "--freeze", "--name",
      "demo", "--namespace", "default"]'
    pre.hook.backup.velero.io/container: compute
  creationTimestamp: "2022-09-14T00:38:33Z"
  generateName: virt-launcher-demo-
  labels:
    harvesterhci.io/vmName: demo
    kubevirt.io: virt-launcher
    kubevirt.io/created-by: aac87e41-7f7d-4d75-a641-d00e3951cabc
  name: virt-launcher-demo-clvzw
  namespace: default
  ownerReferences:
  - apiVersion: kubevirt.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: VirtualMachineInstance
    name: demo
    uid: aac87e41-7f7d-4d75-a641-d00e3951cabc
  resourceVersion: "56901506"
  uid: 6c5fa21c-ad21-4e6c-a49d-352d1c8b45d6
spec:
  automountServiceAccountToken: false
  containers:
  - command:
    - /usr/bin/virt-launcher
    - --qemu-timeout
    - 256s
    - --name
    - demo
    - --uid
    - aac87e41-7f7d-4d75-a641-d00e3951cabc
    - --namespace
    - default
    - --kubevirt-share-dir
    - /var/run/kubevirt
    - --ephemeral-disk-dir
    - /var/run/kubevirt-ephemeral-disks
    - --container-disk-dir
    - /var/run/kubevirt/container-disks
    - --grace-period-seconds
    - "45"
    - --hook-sidecars
    - "0"
    - --ovmf-path
    - /usr/share/OVMF
    env:
    - name: KUBEVIRT_RESOURCE_NAME_default
    - name: POD_NAME
      valueFrom:
        fieldRef:
          apiVersion: v1
          fieldPath: metadata.name
    image: registry.suse.com/harvester-beta/virt-launcher:0.49.0-2
    imagePullPolicy: IfNotPresent
    name: compute
    resources:
      limits:
        cpu: "2"
        devices.kubevirt.io/kvm: "1"
        devices.kubevirt.io/tun: "1"
        devices.kubevirt.io/vhost-net: "1"
        memory: "4487204865"
      requests:
        cpu: 125m
        devices.kubevirt.io/kvm: "1"
        devices.kubevirt.io/tun: "1"
        devices.kubevirt.io/vhost-net: "1"
        ephemeral-storage: 50M
        memory: "3054850049"
    securityContext:
      capabilities:
        add:
        - NET_BIND_SERVICE
        - SYS_NICE
        drop:
        - NET_RAW
      privileged: false
      runAsUser: 0
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    volumeDevices:
    - devicePath: /dev/disk-0
      name: disk-0
    - devicePath: /dev/disk-1
      name: disk-1
    volumeMounts:
    - mountPath: /var/run/kubevirt-private
      name: private
    - mountPath: /var/run/kubevirt
      name: public
    - mountPath: /var/run/kubevirt-ephemeral-disks
      name: ephemeral-disks
    - mountPath: /var/run/kubevirt/container-disks
      mountPropagation: HostToContainer
      name: container-disks
    - mountPath: /var/run/kubevirt/hotplug-disks
      mountPropagation: HostToContainer
      name: hotplug-disks
    - mountPath: /var/run/libvirt
      name: libvirt-runtime
    - mountPath: /var/run/kubevirt/sockets
      name: sockets
    - mountPath: /var/run/kubevirt-private/secret/cloudinitdisk/userdata
      name: cloudinitdisk-udata
      readOnly: true
      subPath: userdata
    - mountPath: /var/run/kubevirt-private/secret/cloudinitdisk/userData
      name: cloudinitdisk-udata
      readOnly: true
      subPath: userData
    - mountPath: /var/run/kubevirt-private/secret/cloudinitdisk/networkdata
      name: cloudinitdisk-ndata
      readOnly: true
      subPath: networkdata
    - mountPath: /var/run/kubevirt-private/secret/cloudinitdisk/networkData
      name: cloudinitdisk-ndata
      readOnly: true
      subPath: networkData
  dnsPolicy: ClusterFirst
  enableServiceLinks: false
  hostname: demo
  nodeName: harvester-29sj6
  nodeSelector:
    kubevirt.io/schedulable: "true"
  preemptionPolicy: PreemptLowerPriority
  priority: 0
  readinessGates:
  - conditionType: kubevirt.io/virtual-machine-unpaused
  restartPolicy: Never
  schedulerName: default-scheduler
  securityContext:
    runAsUser: 0
    seLinuxOptions:
      type: virt_launcher.process
  serviceAccount: default
  serviceAccountName: default
  terminationGracePeriodSeconds: 60
  tolerations:
  - effect: NoExecute
    key: node.kubernetes.io/not-ready
    operator: Exists
    tolerationSeconds: 300
  - effect: NoExecute
    key: node.kubernetes.io/unreachable
    operator: Exists
    tolerationSeconds: 300
  volumes:
  - emptyDir: {}
    name: private
  - emptyDir: {}
    name: public
  - emptyDir: {}
    name: sockets
  - name: disk-0
    persistentVolumeClaim:
      claimName: demo-disk-0-wlu7e
  - name: disk-1
    persistentVolumeClaim:
      claimName: demo-disk-1-juxx0
  - name: cloudinitdisk-udata
    secret:
      defaultMode: 420
      secretName: demo-lp2fp
  - name: cloudinitdisk-ndata
    secret:
      defaultMode: 420
      secretName: demo-lp2fp
  - emptyDir: {}
    name: virt-bin-share-dir
  - emptyDir: {}
    name: libvirt-runtime
  - emptyDir: {}
    name: ephemeral-disks
  - emptyDir: {}
    name: container-disks
  - emptyDir: {}
    name: hotplug-disks
`
)

func generateObjects() ([]runtime.Object, error) {
	objs, err := yaml.ToObjects(strings.NewReader(DefaultPod))
	return objs, err
}

func generateFakeClient() (client.WithWatch, error) {
	objs, err := generateObjects()
	if err != nil {
		return nil, err
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return c, nil
}

func Test_createCapabilitiesPatch(t *testing.T) {
	assert := require.New(t)
	c, err := generateFakeClient()
	assert.NoError(err, "expected no error during generation of fake client")
	p := &corev1.Pod{}
	err = c.Get(context.TODO(), types.NamespacedName{Namespace: "default", Name: "virt-launcher-demo-clvzw"}, p)
	assert.NoError(err, "expect no error during pod fetch")
	patch, err := createCapabilityPatch(p)
	assert.NoError(err, "expected no error during patch generation")
	patchData := fmt.Sprintf("[%s]", strings.Join(patch, ","))
	podJson, err := json.Marshal(p)
	assert.NoError(err, "expected no error during marshalling of pod resource")
	prepPatch, err := jsonpatch.DecodePatch([]byte(patchData))
	assert.NoErrorf(err, "expected no error during preperation of patch")
	newPodJson, err := prepPatch.Apply(podJson)
	assert.NoError(err, "expected no error during application of pod")
	newPod := &corev1.Pod{}
	err = json.Unmarshal(newPodJson, newPod)
	assert.NoError(err, "no error found during conversion of json to pod")
	assert.Len(newPod.Spec.Containers[0].SecurityContext.Capabilities.Add, 3, "expected to find 3 capabilities")
	assert.Equal(newPod.Spec.Containers[0].SecurityContext.Capabilities.Add[2], corev1.Capability("SYS_RESOURCE"))
}
