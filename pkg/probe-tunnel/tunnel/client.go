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

package client

import (
	"context"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/rancher/remotedialer"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
)

var connected = make(chan struct{})

const (
	dailEndPointSuffix = "/clusteragent/connect"
	clusterInfoCm      = "dice-cluster-info"
)

func initClientSet() (client.Client, *kubernetes.Clientset, *rest.Config, error) {
	var err error
	var clientset *kubernetes.Clientset
	var config *rest.Config
	var k8sRestClient client.Client

	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		userHomeDir = ""
	}
	kubeConfig := filepath.Join(userHomeDir, ".kube", "config")
	config, err = rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			klog.Errorf("[remote dialer agent] get kubernetes client config error: %+v\n", err)
			return nil, nil, nil, err
		}
	}

	config.AcceptContentTypes = "application/json"
	if clientset, err = kubernetes.NewForConfig(config); err != nil {
		return nil, nil, nil, err
	}

	scheme := runtime.NewScheme()
	kubeproberv1.AddToScheme(scheme)
	k8sRestClient, err = client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, nil, nil, err
	}

	return k8sRestClient, clientset, config, nil
}

func getClusterName(clientset *kubernetes.Clientset) (string, error) {
	var cm *v1.ConfigMap
	var err error
	if cm, err = clientset.CoreV1().ConfigMaps(metav1.NamespaceDefault).Get(context.Background(), clusterInfoCm, metav1.GetOptions{}); err != nil {
		return "", err
	}
	return cm.Data["DICE_CLUSTER_NAME"], nil
}
func Start(ctx context.Context, cfg *Config) {
	var clusterDialEndpoint string
	var clientset *kubernetes.Clientset
	var clusterName string
	var err error

	if _, clientset, _, err = initClientSet(); err != nil {
		return
	}

	if cfg.ClusterName == "" {
		if clusterName, err = getClusterName(clientset); err != nil {
			klog.Error("[probe tunnel] clusterName is not set or configmaps dice-cluster-info not found")
			return
		}
	} else {
		clusterName = cfg.ClusterName
	}

	headers := http.Header{
		"X-Cluster-Name": {clusterName},
		"Secret-Key":     {cfg.SecretKey},
	}

	u, err := url.Parse(cfg.ProbeMasterAddr)
	if err != nil {
		klog.Errorf("[tunnel-client] get probe-master addr error: %+v\n", err)
		return
	}
	switch u.Scheme {
	case "http":
		clusterDialEndpoint = "ws://" + u.Host + dailEndPointSuffix
	case "https":
		clusterDialEndpoint = "wss://" + u.Host + dailEndPointSuffix
	}

	for {
		remotedialer.ClientConnect(ctx, clusterDialEndpoint, headers, nil, func(proto, address string) bool {
			switch proto {
			case "tcp":
				return true
			case "unix":
				return address == "/var/run/docker.sock"
			case "npipe":
				return address == "//./pipe/docker_engine"
			}
			return false
		}, onConnect)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(rand.Int()%10) * time.Second):
			// retry connect after sleep a random time
		}
	}

}

func onConnect(ctx context.Context, session *remotedialer.Session) error {
	connected <- struct{}{}
	return nil

}

func IsContain(items []string, item string) bool {
	for _, eachItem := range items {
		if eachItem == item {
			return true
		}
	}
	return false
}
