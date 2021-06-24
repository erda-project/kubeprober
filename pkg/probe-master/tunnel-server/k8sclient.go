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

package server

import (
	"os"
	"path/filepath"

	kubeprobev1 "github.com/erda-project/kubeprober/pkg/probe-master/apis/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	clusterRestClient client.Client
)

func init() {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		userHomeDir = ""
	}
	kubeConfig := filepath.Join(userHomeDir, ".kube", "config")
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			klog.Errorf("[remote dialer server] get kubernetes client config error: %+v\n", err)
			return
		}
	}

	scheme := runtime.NewScheme()
	kubeprobev1.AddToScheme(scheme)
	clusterRestClient, err = client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return
	}
}
