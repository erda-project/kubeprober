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
	"github.com/spf13/pflag"
)

type ProbeMasterOptions struct {
	MetricsAddr              string
	PprofAddr                string
	HealthProbeAddr          string
	EnableLeaderElection     bool
	EnablePprof              bool
	LeaderElectionNamespace  string
	Namespace                string
	CreateDefaultPool        bool
	Version                  bool
	ProbeMasterListenAddr    string
	ConfigFile               string
	InfluxdbEnable           bool
	InfluxdbHost             string
	InfluxdbToken            string
	InfluxdbOrg              string
	InfluxdbBucket           string
	AlertDataBucket          string
	ErdaOpenapiURL           string
	ErdaUsername             string
	ErdaPassword             string
	ErdaOrg                  string
	ErdaProjectId            uint64
	ErdaTicketEnable         bool
	BypassPushMetricPassword string
}

// NewProbeMasterOptions creates a new NewProbeMasterOptions with a default config.
func NewProbeMasterOptions() *ProbeMasterOptions {
	o := &ProbeMasterOptions{
		MetricsAddr:              ":8081",
		PprofAddr:                ":8091",
		HealthProbeAddr:          ":8001",
		EnableLeaderElection:     false,
		EnablePprof:              false,
		LeaderElectionNamespace:  "kube-system",
		Namespace:                "",
		CreateDefaultPool:        false,
		ProbeMasterListenAddr:    ":8088",
		ConfigFile:               "",
		InfluxdbEnable:           false,
		ErdaTicketEnable:         false,
		BypassPushMetricPassword: "BypassPushMetricPassword",
	}

	return o
}

// ValidateOptions validates YurtAppOptions
func ValidateOptions(options *ProbeMasterOptions) error {
	// TODO
	return nil
}

// AddFlags returns flags for a specific yurthub by section name
func (o *ProbeMasterOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.MetricsAddr, "metrics-addr", o.MetricsAddr, "The address the metric endpoint binds to.")
	fs.StringVar(&o.PprofAddr, "pprof-addr", o.PprofAddr, "The address the pprof binds to.")
	fs.StringVar(&o.HealthProbeAddr, "health-probe-addr", o.HealthProbeAddr, "The address the healthz/readyz endpoint binds to.")
	fs.BoolVar(&o.EnableLeaderElection, "enable-leader-election", o.EnableLeaderElection, "Whether you need to enable leader election.")
	fs.BoolVar(&o.EnablePprof, "enable-pprof", o.EnablePprof, "Enable pprof for controller manager.")
	fs.StringVar(&o.LeaderElectionNamespace, "leader-election-namespace", o.LeaderElectionNamespace, "This determines the namespace in which the leader election configmap will be created, it will use in-cluster namespace if empty.")
	fs.StringVar(&o.Namespace, "namespace", o.Namespace, "Namespace if specified restricts the manager's cache to watch objects in the desired namespace. Defaults to all namespaces.")
	fs.BoolVar(&o.CreateDefaultPool, "create-default-pool", o.CreateDefaultPool, "Create default cloud/edge pools if indicated.")
	fs.BoolVar(&o.Version, "version", o.Version, "print the version information.")
	fs.StringVar(&o.ConfigFile, "config-file", o.ConfigFile, "read configurations from config file if set.")
	fs.BoolVar(&o.InfluxdbEnable, "influxdb_enable", o.InfluxdbEnable, "if send probe event to influxdb or not.")
	fs.StringVar(&o.InfluxdbHost, "influxdb_host", o.InfluxdbHost, "influxdb host value")
	fs.StringVar(&o.InfluxdbToken, "influxdb_token", o.InfluxdbToken, "influxdb token value.")
	fs.StringVar(&o.InfluxdbOrg, "influxdb_org", o.InfluxdbOrg, "influxdb org value.")
	fs.StringVar(&o.InfluxdbBucket, "influxdb_bucket", o.InfluxdbBucket, "influxdb kucket value.")
	fs.StringVar(&o.AlertDataBucket, "alert_data_kucket", o.AlertDataBucket, "alert data kucket value.")
	fs.BoolVar(&o.ErdaTicketEnable, "erda_ticket_enable", o.ErdaTicketEnable, "if true, send ticket to erda.")
	fs.StringVar(&o.ErdaOpenapiURL, "erda_openapi_url", o.ErdaOpenapiURL, "erda openapi url.")
	fs.StringVar(&o.ErdaUsername, "erda_username", o.ErdaUsername, "erda username.")
	fs.StringVar(&o.ErdaPassword, "erda_password", o.ErdaPassword, "erda password.")
	fs.StringVar(&o.ErdaOrg, "erda_org", o.ErdaOrg, "erda organization.")
	fs.Uint64Var(&o.ErdaProjectId, "erda_project_id", o.ErdaProjectId, "erda project id.")
}
