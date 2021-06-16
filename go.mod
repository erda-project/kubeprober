module github.com/erda-project/kubeprober

go 1.15

require (
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/go-logr/logr v0.3.0
	github.com/gorilla/mux v1.8.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/rancher/remotedialer v0.0.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.6.1 // indirect
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/klog v1.0.0
	sigs.k8s.io/controller-runtime v0.8.3
)

replace github.com/rancher/remotedialer => github.com/erda-project/remotedialer v0.2.6-0.20210518122121-2ff7d3d4deea
