// Copyright (c) 2021 Terminus, Inc.
//
// This program is free software: you can use, redistribute, and/or modify
// it under the terms of the GNU Affero General Public License, version 3
// or later ("AGPL"), as published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package options

import (
	"fmt"
	"net/url"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/spf13/pflag"

	probev1alpha1 "github.com/erda-project/kubeprober/pkg/probe-agent/apis/v1alpha1"
)

var ProbeAgentConf = NewProbeAgentOptions()

type ProbeAgentOptions struct {
	MetricsAddr             string
	PprofAddr               string
	HealthProbeAddr         string
	EnableLeaderElection    bool
	EnablePprof             bool
	LeaderElectionNamespace string
	Namespace               string
	CreateDefaultPool       bool
	Version                 bool
	ProbeMasterAddr         string
	ClusterName             string
	SecretKey               string
	ProbeStatusReportUrl    string
	ProbeListenAddr         string
	ProbeAgentDebug         string
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
		Namespace:               "",
		CreateDefaultPool:       false,
		ProbeListenAddr:         ":8082",
		ProbeStatusReportUrl:    "http://probeagent.default.svc.cluster.local/probe-status",
	}

	return o
}

// ValidateOptions validates YurtAppOptions
func (o *ProbeAgentOptions) ValidateOptions() error {
	// TODO
	// ProbeStatusReportUrl Validate
	_, err := url.ParseRequestURI(o.ProbeStatusReportUrl)
	if err != nil {
		err := fmt.Errorf("parse ProbeStatusReportUrl failed, error:%v", err)
		return err
	}
	return nil
}

func (o *ProbeAgentOptions) PostConfig() error {
	ns := os.Getenv("POD_NAMESPACE")
	if o.ProbeStatusReportUrl == "" && ns == "" {
		err := fmt.Errorf("both ProbeStatusReportUrl and POD_NAMESPACE environment is empty")
		return err
	}
	if o.ProbeStatusReportUrl == "" {
		o.ProbeStatusReportUrl = fmt.Sprintf("http://probeagent.%s.svc.cluster.local%s/probe-status", ns, o.ProbeListenAddr)
	}
	logrus.Infof("probe status report url %s", o.ProbeStatusReportUrl)
	return nil
}

func (o ProbeAgentOptions) GetProbeStatusReportUrl() string {
	return o.ProbeStatusReportUrl
}

// AddFlags returns flags for a specific yurthub by section name
func (o *ProbeAgentOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.MetricsAddr, "metrics-addr", o.MetricsAddr, "The address the metric endpoint binds to.")
	fs.StringVar(&o.PprofAddr, "pprof-addr", o.PprofAddr, "The address the pprof binds to.")
	fs.StringVar(&o.HealthProbeAddr, "health-probe-addr", o.HealthProbeAddr, "The address the healthz/readyz endpoint binds to.")
	fs.BoolVar(&o.EnableLeaderElection, "enable-leader-election", o.EnableLeaderElection, "Whether you need to enable leader election.")
	fs.BoolVar(&o.EnablePprof, "enable-pprof", o.EnablePprof, "Enable pprof for controller manager.")
	fs.StringVar(&o.LeaderElectionNamespace, "leader-election-namespace", o.LeaderElectionNamespace, "This determines the namespace in which the leader election configmap will be created, it will use in-cluster namespace if empty.")
	fs.StringVar(&o.Namespace, "namespace", o.Namespace, "Namespace if specified restricts the manager's cache to watch objects in the desired namespace. Defaults to all namespaces.")
	fs.BoolVar(&o.CreateDefaultPool, "create-default-pool", o.CreateDefaultPool, "Create default cloud/edge pools if indicated.")
	fs.BoolVar(&o.Version, "version", o.Version, "print the version information.")
	fs.StringVar(&o.ProbeMasterAddr, "probe-master-addr", os.Getenv("PROBE_MASTER_ADDR"), "The address of the probe-master")
	fs.StringVar(&o.ClusterName, "cluster-name", os.Getenv("CLUSTER_NAME"), "cluster name.")
	fs.StringVar(&o.SecretKey, "secret-key", os.Getenv("SECRET_KEY"), "secret key of this cluster.")
	fs.StringVar(&o.ProbeStatusReportUrl, "probestatus-report-url", os.Getenv(probev1alpha1.ProbeStatusReportUrl), "probe status report url for probe check pod")
	fs.StringVar(&o.ProbeAgentDebug, "probe-agent-debug", os.Getenv("PROBE_AGENT_DEBUG"), "whether debug probe agent, if debug stop tunnel service")
}
