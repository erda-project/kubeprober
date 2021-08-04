package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/erda-project/kubeprober/pkg/envconf"
)

type Conf struct {
	// default images used for check.
	CheckImage              string        `env:"CHECK_IMAGE" default:"nginxinc/nginx-unprivileged:1.17.8"`
	CheckDeploymentName     string        `env:"CHECK_DEPLOYMENT_NAME" default:"deploy-service-check"`
	CheckServiceName        string        `env:"CHECK_SERVICE_NAME" default:"deploy-service-check"`
	CheckContainerPort      int32         `env:"CHECK_CONTAINER_PORT" default:"8080"`
	CheckLoadBalancerPort   int32         `env:"CHECK_LOAD_BALANCER_PORT" default:"80"`
	CheckNamespace          string        `env:"CHECK_NAMESPACE" default:"kubeprober-deploy-service-check"`
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

	// common config
	CheckTimeout   time.Duration `env:"CHECK_TIMEOUT" default:"15m"`
	KubeConfigFile string        `env:"KUBECONFIG_FILE"`
	Debug          bool          `env:"DEBUG" default:"false"`

	CheckDeploymentToleration     []apiv1.Toleration
	CheckDeploymentNodeSelectors  map[string]string
	CheckDeploymentAdditionalEnvs map[string]string
}

var cfg Conf

// Load 从环境变量加载配置选项.
func ConfigLoad() {
	envconf.MustLoad(&cfg)
}

// parseInputValues parses all incoming environment variables for the program into globals and fatals on errors.
func ParseConfig() error {

	cfg.CheckNamespace = fmt.Sprintf("%s-%v", cfg.CheckNamespace, time.Now().Unix())

	// Parse incoming deployment toleration
	if len(cfg.CheckTolerationEnvs) > 0 {
		cfg.CheckDeploymentToleration = make([]apiv1.Toleration, 0)
		splitEnvVars := strings.Split(cfg.CheckTolerationEnvs, ",")
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
				cfg.CheckDeploymentToleration = append(cfg.CheckDeploymentToleration, t)
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
				cfg.CheckDeploymentToleration = append(cfg.CheckDeploymentToleration, t)
				continue
			}
			t := corev1.Toleration{
				Key:      parsedEnvVarKeyValuePair[0],
				Operator: corev1.TolerationOpEqual,
				Value:    parsedEnvVarValueEffect[0],
				Effect:   corev1.TaintEffect(parsedEnvVarValueEffect[1]),
			}
			logrus.Infoln("Adding toleration to deployment:", t)
			cfg.CheckDeploymentToleration = append(cfg.CheckDeploymentToleration, t)
		}
		logrus.Infoln("Parsed TOLERATION:", cfg.CheckDeploymentToleration)
	}

	// Parse incoming deployment node selectors
	if len(cfg.CheckNodeSelectorsEnvs) > 0 {
		cfg.CheckDeploymentNodeSelectors = make(map[string]string)
		splitEnvVars := strings.Split(cfg.CheckNodeSelectorsEnvs, ",")
		for _, splitEnvVarKeyValuePair := range splitEnvVars {
			parsedEnvVarKeyValuePair := strings.Split(splitEnvVarKeyValuePair, "=")
			if len(parsedEnvVarKeyValuePair) != 2 {
				logrus.Warnln("Unable to parse key value pair:", splitEnvVarKeyValuePair)
				continue
			}
			if _, ok := cfg.CheckDeploymentNodeSelectors[parsedEnvVarKeyValuePair[0]]; !ok {
				cfg.CheckDeploymentNodeSelectors[parsedEnvVarKeyValuePair[0]] = parsedEnvVarKeyValuePair[1]
			}
		}
		logrus.Infoln("Parsed NODE_SELECTOR:", cfg.CheckDeploymentNodeSelectors)
	}

	// Parse incoming container environment variables
	// (in case custom used images require additional environment variables)
	if len(cfg.CheckAdditionalEnvs) != 0 {
		cfg.CheckDeploymentAdditionalEnvs = make(map[string]string)
		splitEnvVars := strings.Split(cfg.CheckAdditionalEnvs, ",")
		for _, splitEnvVarKeyValuePair := range splitEnvVars {
			parsedEnvVarKeyValuePair := strings.Split(splitEnvVarKeyValuePair, "=")
			if _, ok := cfg.CheckDeploymentAdditionalEnvs[parsedEnvVarKeyValuePair[0]]; !ok {
				cfg.CheckDeploymentAdditionalEnvs[parsedEnvVarKeyValuePair[0]] = parsedEnvVarKeyValuePair[1]
			}
		}
		logrus.Infoln("Parsed ADDITIONAL_ENVS:", cfg.CheckDeploymentAdditionalEnvs)
	}

	return nil
}
