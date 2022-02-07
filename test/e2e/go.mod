module sigs.k8s.io/cluster-api-provider-packet/test/e2e

go 1.16

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/docker/distribution v2.7.1+incompatible
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.16.0
	github.com/packethost/packngo v0.21.0
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	k8s.io/api v0.22.6
	k8s.io/apimachinery v0.22.6
	k8s.io/client-go v0.22.6
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	sigs.k8s.io/cluster-api v1.0.4
	sigs.k8s.io/cluster-api-provider-packet v0.0.0-00010101000000-000000000000
	sigs.k8s.io/cluster-api/test v1.0.4
	sigs.k8s.io/controller-runtime v0.10.3
)

replace (
	sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v1.0.4
	sigs.k8s.io/cluster-api-provider-packet => ../../
)
