// Copyright (c) 2021 Terminus, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package options

import (
	"fmt"
	"net/url"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

const (
	MetricsAddrFlag             = "metrics-addr"
	PprofAddrFlag               = "pprof-addr"
	HealthProbeAddrFlag         = "health-probe-addr"
	EnableLeaderElectionFlag    = "enable-leader-election"
	EnablePprofFlag             = "enable-pprof"
	ProbeMasterAddrFalg         = "probe-master-addr"
	ClusterNameFalg             = "cluster-name"
	LeaderElectionNamespaceFlag = "leader-election-namespace"
	NamespaceFlag               = "namespace"
	ProbeStatusReportUrlFalg    = "probestatus-report-url"
	ProbeListenAddrFalg         = "probe-listen-addr"
	DebugLevelFalg              = "debug-level"
	ConfigFileFalg              = "config-file"
)

var ProbeAgentConf = NewProbeAgentOptions()

type ProbeAgentOptions struct {
	MetricsAddr             string `mapstructure:"metrics_addr" yaml:"metrics_addr"`
	PprofAddr               string `mapstructure:"pprof_addr" yaml:"pprof_addr"`
	HealthProbeAddr         string `mapstructure:"health_probe_addr" yaml:"health_probe_addr"`
	EnableLeaderElection    bool   `mapstructure:"enable_leader_election" yaml:"enable_leader_election"`
	EnablePprof             bool   `mapstructure:"enable_pprof" yaml:"enable_pprof"`
	LeaderElectionNamespace string `mapstructure:"leader_election_namespace" yaml:"leader_election_namespace"`
	LeaderElectionID        string `mapstructure:"leader_election_id" yaml:"leader_election_id"`
	Namespace               string `mapstructure:"namespace" yaml:"namespace"`
	ProbeMasterAddr         string `mapstructure:"probe_master_addr" yaml:"probe_master_addr"`
	ClusterName             string `mapstructure:"cluster_name" yaml:"cluster_name"`
	ProbeStatusReportUrl    string `mapstructure:"probe_status_report_url" yaml:"probe_status_report_url"`
	ProbeListenAddr         string `mapstructure:"probe_listen_addr" yaml:"probe_listen_addr"`
	DebugLevel              int8   `mapstructure:"debug_level" yaml:"debug_level"`
	ConfigFile              string
}

// NewProbeAgentOptions creates a new NewProbeAgentOptions with a default config.
func NewProbeAgentOptions() *ProbeAgentOptions {
	o := &ProbeAgentOptions{
		MetricsAddr:             ":8080",
		PprofAddr:               ":8090",
		HealthProbeAddr:         ":8000",
		EnableLeaderElection:    false,
		EnablePprof:             false,
		LeaderElectionNamespace: "kube-system",
		LeaderElectionID:        "88d8007a.erda.cloud",
		Namespace:               "default",
		ProbeListenAddr:         ":8082",
		ProbeStatusReportUrl:    "",
		DebugLevel:              1,
		ConfigFile:              "",
	}

	return o
}

// ValidateOptions validates YurtAppOptions
func (o *ProbeAgentOptions) ValidateOptions() error {
	_, err := url.ParseRequestURI(o.ProbeStatusReportUrl)
	if err != nil {
		err := fmt.Errorf("parse ProbeStatusReportUrl failed, error:%v", err)
		return err
	}
	if o.Namespace == "" {
		err := fmt.Errorf("empty namespace")
		return err
	}
	return nil
}

func (o *ProbeAgentOptions) LoadConfig() error {

	// pod running namespace with higher priority
	ns := os.Getenv("POD_NAMESPACE")
	if ns != "" {
		o.Namespace = ns
	}
	if o.ProbeStatusReportUrl == "" && o.Namespace == "" {
		err := fmt.Errorf("both probe_status_report_url and namespace environment is empty")
		return err
	}
	if o.ProbeStatusReportUrl == "" {
		o.ProbeStatusReportUrl = fmt.Sprintf("http://probeagent.%s.svc.cluster.local%s/probe-status", o.Namespace, o.ProbeListenAddr)
	}
	logrus.Infof("probe-agent config: %+v", o)
	return nil
}

func (o ProbeAgentOptions) GetProbeStatusReportUrl() string {
	return o.ProbeStatusReportUrl
}

func (o ProbeAgentOptions) GetNamespace() string {
	return o.Namespace
}

// AddFlags returns flags for a specific yurthub by section name
func (o *ProbeAgentOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.MetricsAddr, MetricsAddrFlag, o.MetricsAddr, "The address the metric endpoint binds to.")
	fs.StringVar(&o.PprofAddr, PprofAddrFlag, o.PprofAddr, "The address the pprof binds to.")
	fs.StringVar(&o.HealthProbeAddr, HealthProbeAddrFlag, o.HealthProbeAddr, "The address the healthz/readyz endpoint binds to.")
	fs.StringVar(&o.ProbeMasterAddr, ProbeMasterAddrFalg, o.ProbeMasterAddr, "The address of the probe-master")
	fs.StringVar(&o.ClusterName, ClusterNameFalg, o.ClusterName, "cluster name.")
	fs.BoolVar(&o.EnableLeaderElection, EnableLeaderElectionFlag, o.EnableLeaderElection, "Whether you need to enable leader election.")
	fs.BoolVar(&o.EnablePprof, EnablePprofFlag, o.EnablePprof, "Enable pprof for controller manager.")
	fs.StringVar(&o.LeaderElectionNamespace, LeaderElectionNamespaceFlag, o.LeaderElectionNamespace, "This determines the namespace in which the leader election configmap will be created, it will use in-cluster namespace if empty.")
	fs.StringVar(&o.Namespace, NamespaceFlag, o.Namespace, "Namespace if specified restricts the manager's cache to watch objects in the desired namespace. Defaults to default namespace.")
	fs.StringVar(&o.ProbeStatusReportUrl, ProbeStatusReportUrlFalg, o.ProbeStatusReportUrl, "probe status report url for probe check pod")
	fs.StringVar(&o.ProbeListenAddr, ProbeListenAddrFalg, o.ProbeListenAddr, "probe agent listen address")
	fs.Int8Var(&o.DebugLevel, DebugLevelFalg, o.DebugLevel, "a debug level is a logging priority. higher levels meaning more debug log.")
	fs.StringVar(&o.ConfigFile, ConfigFileFalg, o.ConfigFile, "read configurations from config file if set.")
}
