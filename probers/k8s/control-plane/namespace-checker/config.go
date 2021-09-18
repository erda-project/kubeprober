package deployment_service_checker

import (
	"github.com/erda-project/kubeprober/probers/k8s/control-plane/config"
)

const (
	CheckNewNamespace = "kubeprober-namespace-check"
)

var cfg *config.Conf

func init() {
	cfg = &config.Cfg
}
