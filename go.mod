module github.com/erda-project/kubeprober

go 1.16

require (
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/go-logr/logr v0.4.0
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/pkg/errors v0.9.1
	github.com/rancher/remotedialer v0.0.0
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/klog v1.0.0
	sigs.k8s.io/controller-runtime v0.9.0
)

replace github.com/rancher/remotedialer => github.com/erda-project/remotedialer v0.2.6-0.20210518122121-2ff7d3d4deea
