package fluentbit_checker

import "github.com/erda-project/kubeprober/probers/k8s/control-plane/config"

var cfg *config.Conf

func init() {
	cfg = &config.Cfg
}
