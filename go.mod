module github.com/harvester/pcidevices

go 1.21

require (
	github.com/evanphx/json-patch v5.6.0+incompatible
	github.com/fsnotify/fsnotify v1.6.0
	github.com/gorilla/mux v1.8.0
	github.com/harvester/harvester v1.2.1
	github.com/harvester/harvester-network-controller v0.3.1
	github.com/jaypipes/ghw v0.9.0
	github.com/jaypipes/pcidb v1.0.0
	github.com/onsi/ginkgo/v2 v2.9.2
	github.com/onsi/gomega v1.27.6
	github.com/rancher/dynamiclistener v0.3.6
	github.com/rancher/lasso v0.0.0-20230830164424-d684fdeb6f29
	github.com/rancher/wrangler v1.1.1
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.8.4
	github.com/u-root/u-root v7.0.0+incompatible
	github.com/urfave/cli/v2 v2.11.1
	github.com/vishvananda/netlink v1.2.1-beta.2
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74
	google.golang.org/grpc v1.60.1
	k8s.io/api v0.28.5
	k8s.io/apimachinery v0.28.5
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-aggregator v0.25.4
	kubevirt.io/client-go v0.54.0
	kubevirt.io/kubevirt v0.55.1
	sigs.k8s.io/controller-runtime v0.13.1
)

require (
	emperror.dev/errors v0.8.0 // indirect
	github.com/banzaicloud/logging-operator/pkg/sdk v0.8.16 // indirect
	github.com/banzaicloud/operator-tools v0.28.10 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/coreos/prometheus-operator v0.38.1-0.20200424145508-7e176fda06cc // indirect
	github.com/emicklei/go-restful/v3 v3.9.0 // indirect
	github.com/go-kit/kit v0.10.0 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gobuffalo/flect v0.2.5 // indirect
	github.com/golang/glog v1.1.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/pprof v0.0.0-20230817174616-7a8ec2ada47b // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/iancoleman/orderedmap v0.2.0 // indirect
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v0.0.0-20200331171230-d50e42f2b669 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/klauspost/pgzip v1.2.4 // indirect
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.2.0 // indirect
	github.com/longhorn/longhorn-manager v1.6.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v0.0.0 // indirect
	github.com/openshift/client-go v0.0.0 // indirect
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.62.0 // indirect
	github.com/rancher/aks-operator v1.0.7 // indirect
	github.com/rancher/eks-operator v1.1.5 // indirect
	github.com/rancher/fleet/pkg/apis v0.0.0-20230123175930-d296259590be // indirect
	github.com/rancher/gke-operator v1.1.4 // indirect
	github.com/rancher/norman v0.0.0-20221205184727-32ef2e185b99 // indirect
	github.com/rancher/rancher/pkg/apis v0.0.0 // indirect
	github.com/rancher/rke v1.3.18 // indirect
	github.com/rancher/system-upgrade-controller/pkg/apis v0.0.0-20210727200656-10b094e30007 // indirect
	github.com/spf13/cast v1.5.1 // indirect
	gomodules.xyz/jsonpatch/v2 v2.2.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240108191215-35c7eff3a6b1 // indirect
	howett.net/plist v1.0.0 // indirect
	k8s.io/apiserver v0.28.5 // indirect
	k8s.io/component-base v0.28.5 // indirect
	sigs.k8s.io/cli-utils v0.27.0 // indirect
	sigs.k8s.io/cluster-api v1.2.0-beta.0 // indirect
)

require (
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/zapr v1.2.3 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.5.0
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/openshift/custom-resource-status v1.1.2 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.17.0 // indirect
	github.com/prometheus/client_model v0.4.1-0.20230718164431-9a2bf3000d16 // indirect
	github.com/prometheus/common v0.44.0 // indirect
	github.com/prometheus/procfs v0.11.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/ulikunitz/xz v0.5.8 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
	golang.org/x/crypto v0.18.0 // indirect
	golang.org/x/mod v0.13.0 // indirect
	golang.org/x/net v0.20.0
	golang.org/x/oauth2 v0.13.0 // indirect
	golang.org/x/sync v0.4.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
	golang.org/x/term v0.16.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.14.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/protobuf v1.32.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.25.4 // indirect
	k8s.io/code-generator v0.25.4 // indirect
	k8s.io/gengo v0.0.0-20220613173612-397b4ae3bce7 // indirect
	k8s.io/klog v1.0.0 // indirect
	k8s.io/klog/v2 v2.100.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230717233707-2695361300d9 // indirect
	k8s.io/kubernetes v1.28.5
	k8s.io/utils v0.0.0-20230505201702-9f6742963106 // indirect
	kubevirt.io/api v0.54.0
	kubevirt.io/containerized-data-importer-api v1.50.0 // indirect
	kubevirt.io/controller-lifecycle-operator-sdk/api v0.0.0-20220329064328-f3cc58c6ed90 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace (
	github.com/emicklei/go-restful => github.com/emicklei/go-restful v2.16.0+incompatible
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.5.5
	github.com/harvester/harvester-network-controller => github.com/harvester/harvester-network-controller v0.3.2-rc1
	github.com/openshift/api => github.com/openshift/api v0.0.0-20191219222812-2987a591a72c
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20200521150516-05eb9880269c
	github.com/rancher/rancher => github.com/rancher/rancher v0.0.0-20230124173128-2207cfed1803
	github.com/rancher/rancher/pkg/apis => github.com/rancher/rancher/pkg/apis v0.0.0-20230124173128-2207cfed1803
	github.com/rancher/rancher/pkg/client => github.com/rancher/rancher/pkg/client v0.0.0-20230124173128-2207cfed1803
	github.com/u-root/u-root => github.com/u-root/u-root v0.10.0
	golang.org/x/net => golang.org/x/net v0.17.0
	golang.org/x/text => golang.org/x/text v0.3.8
	google.golang.org/grpc => google.golang.org/grpc v1.56.3
	k8s.io/api => k8s.io/api v0.24.10
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.24.10
	k8s.io/apimachinery => k8s.io/apimachinery v0.24.10
	k8s.io/apiserver => k8s.io/apiserver v0.24.10
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.24.10
	k8s.io/client-go => k8s.io/client-go v0.24.10
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.24.10
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.24.10
	k8s.io/code-generator => k8s.io/code-generator v0.24.10
	k8s.io/component-base => k8s.io/component-base v0.24.10
	k8s.io/component-helpers => k8s.io/component-helpers v0.24.10
	k8s.io/controller-manager => k8s.io/controller-manager v0.24.10
	k8s.io/cri-api => k8s.io/cri-api v0.24.10
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.24.10
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.24.10
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.24.10
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20220803162953-67bda5d908f1
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.24.10
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.24.10
	k8s.io/kubectl => k8s.io/kubectl v0.24.2
	k8s.io/kubelet => k8s.io/kubelet v0.24.10
	k8s.io/kubernetes => k8s.io/kubernetes v1.24.10
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.24.10
	k8s.io/metrics => k8s.io/metrics v0.24.10
	k8s.io/mount-utils => k8s.io/mount-utils v0.24.10
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.24.10
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.24.10
	kubevirt.io/api => github.com/kubevirt/api v0.59.2
	kubevirt.io/client-go => github.com/kubevirt/client-go v0.54.0
	kubevirt.io/containerized-data-importer-api => kubevirt.io/containerized-data-importer-api v1.47.0
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.11.2
)
