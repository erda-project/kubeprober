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

const (
	EnablePprofFlag     = "enable-pprof"
	ProbeMasterAddrFalg = "probe-master-addr"
	ClusterNameFalg     = "cluster-name"
	SecretKeyFalg       = "secret-key"
	DebugLevelFalg      = "debug-level"
	ConfigFileFalg      = "config-file"
)

var ProbeTunnelConf = NewProbeTunnelOptions()

type ProbeTunnelOptions struct {
	ProbeMasterAddr string `mapstructure:"probe_master_addr" yaml:"probe_master_addr"`
	ClusterName     string `mapstructure:"cluster_name" yaml:"cluster_name"`
	SecretKey       string `mapstructure:"secret_key" yaml:"secret_key"`
	ConfigFile      string
}

// NewProbeTunnelOptions creates a new NewProbeAgentOptions with a default config.
func NewProbeTunnelOptions() *ProbeTunnelOptions {
	o := &ProbeTunnelOptions{
		ConfigFile: "",
	}

	return o
}

// AddFlags returns flags for a specific yurthub by section name
func (o *ProbeTunnelOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.ProbeMasterAddr, ProbeMasterAddrFalg, o.ProbeMasterAddr, "The address of the probe-master")
	fs.StringVar(&o.ClusterName, ClusterNameFalg, o.ClusterName, "cluster name.")
	fs.StringVar(&o.SecretKey, SecretKeyFalg, o.SecretKey, "secret key of this cluster.")
	fs.StringVar(&o.ConfigFile, ConfigFileFalg, o.ConfigFile, "read configurations from config file if set.")
}
