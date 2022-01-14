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

package k8sclient

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
)

var (
	RestClient client.Client
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
	kubeproberv1.AddToScheme(scheme)
	clientgoscheme.AddToScheme(scheme)
	RestClient, err = client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return
	}
}

func GetClusters() ([]kubeproberv1.Cluster, error) {
	clusters := &kubeproberv1.ClusterList{}

	err := RestClient.List(context.Background(), clusters, client.InNamespace(metav1.NamespaceDefault))
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("Get clusters in namespaces %s failed", metav1.NamespaceDefault))
	}
	return clusters.Items, nil
}

func GetCluster(name string) (*kubeproberv1.Cluster, error) {
	clusters, err := GetClusters()
	if err != nil {
		return nil, err
	}

	for _, c := range clusters {
		if c.Name == name {
			return &c, nil
		}
	}

	return nil, nil
}
