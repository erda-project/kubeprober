package main

import (
	"time"

	"github.com/erda-project/kubeprober/pkg/envconf"
)

type Conf struct {
	// dns check config
	PublicDomain  string `env:"PUBLIC_DOMAIN" default:"www.baidu.com" doc:"public domain resolution check"`
	PrivateDomain string `env:"HOSTNAME" default:"kubernetes.default" doc:"inner k8s service domain resolution check"`
	LabelSelector string `env:"LABEL_SELECTOR" default:"k8s-app=kube-dns" doc:"dns label selector"`
	Namespace     string `env:"NAMESPACE" default:"kube-system" doc:"dns namespace"`

	// common config
	CheckTimeout   time.Duration `env:"CHECK_TIMEOUT" default:"5m"`
	KubeConfigFile string        `env:"KUBECONFIG_FILE"`
	Debug          bool          `env:"DEBUG" default:"false"`
}

var cfg Conf

// Load 从环境变量加载配置选项.
func ConfigLoad() {
	envconf.MustLoad(&cfg)
}
