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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rancher/remotedialer"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/apistructs"
)

var connected = make(chan struct{})

const (
	dailEndPointSuffix      = "/clusteragent/connect"
	heartBeatEndPointSuffix = "/heartbeat"
	clusterInfoCm           = "dice-cluster-info"
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

func sendHeartBeat(heartBeatAddr string, clusterName string, secretKey string) error {
	ctx := context.Background()
	var rsp *http.Response
	var err error
	var clientset *kubernetes.Clientset
	var config *rest.Config
	var version *version.Info
	var nodes *v1.NodeList
	var checkerStatus string
	var k8sRestClient client.Client

	if k8sRestClient, clientset, config, err = initClientSet(); err != nil {
		return err
	}
	if version, err = clientset.ServerVersion(); err != nil {
		return err
	}
	if nodes, err = clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err != nil {
		return err
	}
	if clusterName == "" {
		if clusterName, err = getClusterName(clientset); err != nil {
			klog.Error("[heartbeat] clusterName is not set or configmaps dice-cluster-info not found")
			return err
		}
	}

	if checkerStatus, err = getCheckerStatus(k8sRestClient); err != nil {
		return err
	}

	hbData := apistructs.HeartBeatReq{
		Name:           clusterName,
		SecretKey:      secretKey,
		Address:        config.Host,
		ProbeNamespace: os.Getenv("POD_NAMESPACE"),
		CaData:         base64.StdEncoding.EncodeToString(config.CAData),
		CertData:       base64.StdEncoding.EncodeToString(config.CertData),
		KeyData:        base64.StdEncoding.EncodeToString(config.KeyData),
		Token:          base64.StdEncoding.EncodeToString([]byte(config.BearerToken)),
		Version:        version.String(),
		NodeCount:      len(nodes.Items),
		Checkers:       checkerStatus,
	}
	json_data, _ := json.Marshal(hbData)
	if rsp, err = http.Post(heartBeatAddr, "application/json", bytes.NewBuffer(json_data)); err != nil {
		return err
	}
	body, _ := ioutil.ReadAll(rsp.Body)
	if rsp.StatusCode != http.StatusOK {
		return errors.New(string(body))
	}
	rsp.Body.Close()
	return nil
}

func getCheckerStatus(k8sRestClient client.Client) (string, error) {
	var err error
	var probeNames []string
	var totalChecker int
	var ErrorChecker int
	probeStatus := &kubeproberv1.ProbeStatusList{}
	probes := &kubeproberv1.ProbeList{}
	if err = k8sRestClient.List(context.Background(), probeStatus, client.InNamespace("kubeprober")); err != nil {
		return "", err
	}

	if err = k8sRestClient.List(context.Background(), probes, client.InNamespace("kubeprober")); err != nil {
		return "", err
	}

	for _, i := range probes.Items {
		probeNames = append(probeNames, i.Name)
	}
	for _, i := range probeStatus.Items {
		if IsContain(probeNames, i.Name) {
			for _, j := range i.Spec.Checkers {
				totalChecker++
				if j.Status == kubeproberv1.CheckerStatusError {
					ErrorChecker++
				}
			}
		}
	}
	checkerStatus := fmt.Sprintf("%s/%s", strconv.Itoa(totalChecker), strconv.Itoa(ErrorChecker))
	return checkerStatus, nil
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
	var clusterHeartBeatEndpoint string
	headers := http.Header{
		"X-Cluster-Name": {cfg.ClusterName},
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
		clusterHeartBeatEndpoint = "http://" + u.Host + heartBeatEndPointSuffix
	case "https":
		clusterDialEndpoint = "wss://" + u.Host + dailEndPointSuffix
		clusterHeartBeatEndpoint = "https://" + u.Host + heartBeatEndPointSuffix
	}

	go func() {
		for {
			select {
			case <-time.After(30 * time.Second):
				if err := sendHeartBeat(clusterHeartBeatEndpoint, cfg.ClusterName, cfg.SecretKey); err != nil {
					klog.Errorf("[heartbeat] send heartbeat request error: %+v\n", err)
					break
				}
			}
		}
	}()
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
