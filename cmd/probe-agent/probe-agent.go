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

package main

import (
	"flag"
	"math/rand"
	"time"

	"github.com/erda-project/kubeprober/cmd/probe-agent/app"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	klog.InitFlags(nil)
	defer klog.Flush()

	cmd := app.NewCmdProbeAgentManager(wait.NeverStop)
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}
