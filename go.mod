module github.com/spectrocloud/cluster-api-provider-vsphere-static-ip

go 1.13

require (
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/metal3-io/ip-address-manager v0.0.4
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.4.0
	golang.org/x/net v0.0.0-20201021035429-f5854403a974 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	k8s.io/api v0.19.0-alpha.2
	k8s.io/apimachinery v0.19.0-alpha.2
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog v1.0.0
	sigs.k8s.io/cluster-api v0.3.6
	sigs.k8s.io/cluster-api-provider-vsphere v0.6.5
	sigs.k8s.io/controller-runtime v0.6.0
)

replace k8s.io/client-go => k8s.io/client-go v0.19.0-alpha.2
