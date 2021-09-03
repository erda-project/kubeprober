package config

import (
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/erda-project/kubeprober/pkg/envconf"
)

type Conf struct {
	// deployment service check config
	CheckImage            string `env:"CHECK_IMAGE" default:"nginxinc/nginx-unprivileged:1.17.8"`
	CheckDeploymentName   string `env:"CHECK_DEPLOYMENT_NAME" default:"deploy-service-check"`
	CheckServiceName      string `env:"CHECK_SERVICE_NAME" default:"deploy-service-check"`
	CheckContainerPort    int32  `env:"CHECK_CONTAINER_PORT" default:"8080"`
	CheckLoadBalancerPort int32  `env:"CHECK_LOAD_BALANCER_PORT" default:"80"`
	// should be in namespace where probe-agent running
	CheckNamespace          string        `env:"KUBEPROBER_PROBE_NAMESPACE" default:"kubeprober"`
	CheckDeploymentReplicas int           `env:"CHECK_DEPLOYMENT_REPLICAS" default:"1"`
	CheckServiceAccount     string        `env:"CHECK_SERVICE_ACCOUNT" default:"default"`
	CheckTolerationEnvs     string        `env:"CHECK_TOLERATION_ENVS"`
	CheckAdditionalEnvs     string        `env:"CHECK_ADDITIONAL_ENVS"`
	CheckNodeSelectorsEnvs  string        `env:"CHECK_NODE_SELECTOR"`
	CpuRequest              string        `env:"CHECK_POD_CPU_REQUEST" default:"15m"`
	MemoryRequest           string        `env:"CHECK_POD_MEM_REQUEST" default:"20Mi"`
	CpuLimit                string        `env:"CHECK_POD_CPU_LIMIT" default:"75m"`
	MemoryLimit             string        `env:"CHECK_POD_MEM_LIMIT" default:"100Mi"`
	ShutdownGracePeriod     time.Duration `env:"SHUTDOWN_GRACE_PERIOD" default:"30s"`

	// dns check config
	PublicDomain      string `env:"PUBLIC_DOMAIN" default:"www.baidu.com" doc:"public domain resolution check"`
	PrivateDomain     string `env:"PRIVATE_DOMAIN" default:"kubernetes.default" doc:"inner k8s service domain resolution check"`
	DnsLabelSelector  string `env:"DNS_LABEL_SELECTOR" default:"k8s-app=kube-dns" doc:"dns label selector"`
	DnsCheckNamespace string `env:"DNS_CHECK_NAMESPACE" default:"kube-system" doc:"dns namespace"`
	ResourceAutoReap  bool   `env:"RESOURCE_AUTO_REAP" default:"true"`

	// common config
	CheckTimeout   time.Duration `env:"CHECK_TIMEOUT" default:"15m"`
	KubeConfigFile string        `env:"KUBECONFIG_FILE"`
	Debug          bool          `env:"DEBUG" default:"false"`

	CheckDeploymentToleration     []apiv1.Toleration
	CheckDeploymentNodeSelectors  map[string]string
	CheckDeploymentAdditionalEnvs map[string]string
}

var Cfg Conf

// Load 从环境变量加载配置选项.
func Load() {
	envconf.MustLoad(&Cfg)
}

// parseInputValues parses all incoming environment variables for the program into globals and fatals on errors.
func ParseConfig() error {

	// Parse incoming deployment toleration
	if len(Cfg.CheckTolerationEnvs) > 0 {
		Cfg.CheckDeploymentToleration = make([]apiv1.Toleration, 0)
		splitEnvVars := strings.Split(Cfg.CheckTolerationEnvs, ",")
		for _, splitEnvVarKeyValuePair := range splitEnvVars {
			parsedEnvVarKeyValuePair := strings.Split(splitEnvVarKeyValuePair, "=")
			if len(parsedEnvVarKeyValuePair) != 2 {
				logrus.Warnln("Unable to parse key value pair:", splitEnvVarKeyValuePair)
				logrus.Warnln("Setting operator to", corev1.TolerationOpExists)
				t := corev1.Toleration{
					Key:      parsedEnvVarKeyValuePair[0],
					Operator: corev1.TolerationOpExists,
				}
				logrus.Infoln("Adding toleration to deployment:", t)
				Cfg.CheckDeploymentToleration = append(Cfg.CheckDeploymentToleration, t)
				continue
			}
			parsedEnvVarValueEffect := strings.Split(parsedEnvVarKeyValuePair[1], ":")
			if len(parsedEnvVarValueEffect) != 2 {
				logrus.Warnln("Unable to parse complete toleration value and effect:", parsedEnvVarValueEffect)
				t := corev1.Toleration{
					Key:      parsedEnvVarKeyValuePair[0],
					Operator: corev1.TolerationOpEqual,
					Value:    parsedEnvVarKeyValuePair[1],
				}
				logrus.Infoln("Adding toleration to deployment:", t)
				Cfg.CheckDeploymentToleration = append(Cfg.CheckDeploymentToleration, t)
				continue
			}
			t := corev1.Toleration{
				Key:      parsedEnvVarKeyValuePair[0],
				Operator: corev1.TolerationOpEqual,
				Value:    parsedEnvVarValueEffect[0],
				Effect:   corev1.TaintEffect(parsedEnvVarValueEffect[1]),
			}
			logrus.Infoln("Adding toleration to deployment:", t)
			Cfg.CheckDeploymentToleration = append(Cfg.CheckDeploymentToleration, t)
		}
		logrus.Infoln("Parsed TOLERATION:", Cfg.CheckDeploymentToleration)
	}

	// Parse incoming deployment node selectors
	if len(Cfg.CheckNodeSelectorsEnvs) > 0 {
		Cfg.CheckDeploymentNodeSelectors = make(map[string]string)
		splitEnvVars := strings.Split(Cfg.CheckNodeSelectorsEnvs, ",")
		for _, splitEnvVarKeyValuePair := range splitEnvVars {
			parsedEnvVarKeyValuePair := strings.Split(splitEnvVarKeyValuePair, "=")
			if len(parsedEnvVarKeyValuePair) != 2 {
				logrus.Warnln("Unable to parse key value pair:", splitEnvVarKeyValuePair)
				continue
			}
			if _, ok := Cfg.CheckDeploymentNodeSelectors[parsedEnvVarKeyValuePair[0]]; !ok {
				Cfg.CheckDeploymentNodeSelectors[parsedEnvVarKeyValuePair[0]] = parsedEnvVarKeyValuePair[1]
			}
		}
		logrus.Infoln("Parsed NODE_SELECTOR:", Cfg.CheckDeploymentNodeSelectors)
	}

	// Parse incoming container environment variables
	// (in case custom used images require additional environment variables)
	if len(Cfg.CheckAdditionalEnvs) != 0 {
		Cfg.CheckDeploymentAdditionalEnvs = make(map[string]string)
		splitEnvVars := strings.Split(Cfg.CheckAdditionalEnvs, ",")
		for _, splitEnvVarKeyValuePair := range splitEnvVars {
			parsedEnvVarKeyValuePair := strings.Split(splitEnvVarKeyValuePair, "=")
			if _, ok := Cfg.CheckDeploymentAdditionalEnvs[parsedEnvVarKeyValuePair[0]]; !ok {
				Cfg.CheckDeploymentAdditionalEnvs[parsedEnvVarKeyValuePair[0]] = parsedEnvVarKeyValuePair[1]
			}
		}
		logrus.Infoln("Parsed ADDITIONAL_ENVS:", Cfg.CheckDeploymentAdditionalEnvs)
	}

	return nil
}
