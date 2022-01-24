module sigs.k8s.io/cluster-api-provider-packet

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/google/uuid v1.3.0 // indirect
	github.com/onsi/gomega v1.16.0
	github.com/packethost/packngo v0.21.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/term v0.0.0-20210503060354-a79de5458b56 // indirect
	k8s.io/api v0.22.6
	k8s.io/apiextensions-apiserver v0.22.6 // indirect
	k8s.io/apimachinery v0.22.6
	k8s.io/client-go v0.22.6
	k8s.io/component-base v0.22.6
	k8s.io/klog/v2 v2.10.0
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	sigs.k8s.io/cluster-api v1.0.2
	sigs.k8s.io/controller-runtime v0.10.3
)
