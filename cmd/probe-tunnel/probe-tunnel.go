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

package main

import (
	"math/rand"
	"time"

	"github.com/erda-project/kubeprober/cmd/probe-tunnel/app"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	klog.InitFlags(nil)
	defer klog.Flush()

	cmd := app.NewCmdProbeTunnelManager(wait.NeverStop)
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}
