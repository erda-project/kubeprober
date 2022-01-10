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

package heartbeat

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/apistructs"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	heartBeatEndPointSuffix = "/heartbeat"
	clusterInfoCm           = "dice-cluster-info"
)

func Start(ctx context.Context, clusterName string, masterAddr string) {
	var clusterHeartBeatEndpoint string
	var clientset *kubernetes.Clientset
	var name string
	var err error

	klog.Errorf("Begin to get clusterInfo to send heartbeat...")
	if _, clientset, _, err = initClientSet(); err != nil {
		klog.Errorf("[probe tunnel] failed to init k8s clientset: %+v\n", err)
		return
	}
	if clusterName == "" {
		if name, err = getClusterName(clientset); err != nil {
			klog.Error("[probe tunnel] clusterName is not set or configmaps dice-cluster-info not found")
			return
		}
	} else {
		name = clusterName
	}
	u, err := url.Parse(masterAddr)
	if err != nil {
		klog.Errorf("[tunnel-client] get probe-master addr error: %+v\n", err)
		return
	}
	switch u.Scheme {
	case "http":
		clusterHeartBeatEndpoint = "http://" + u.Host + heartBeatEndPointSuffix
	case "https":
		clusterHeartBeatEndpoint = "https://" + u.Host + heartBeatEndPointSuffix
	}
	klog.Errorf("Begin to send heartbeat...")
	klog.Errorf("Cluster name is: %+v\n", name)
	go func() {
		for {
			select {
			case <-time.After(120 * time.Second):
				if err := sendHeartBeat(clusterHeartBeatEndpoint, name); err != nil {
					klog.Errorf("[heartbeat] send heartbeat request error: %+v\n", err)
					break
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

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
	clientgoscheme.AddToScheme(scheme)
	k8sRestClient, err = client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, nil, nil, err
	}

	return k8sRestClient, clientset, config, nil
}

func sendHeartBeat(heartBeatAddr string, clusterName string) error {
	ctx := context.Background()
	var rsp *http.Response
	var err error
	var clientset *kubernetes.Clientset
	var config *rest.Config
	var version *version.Info
	var nodes *v1.NodeList
	var checkerStatus string
	var k8sRestClient client.Client
	var caData []byte
	var extraStatus map[string]string

	if k8sRestClient, clientset, config, err = initClientSet(); err != nil {
		return err
	}
	if version, err = clientset.ServerVersion(); err != nil {
		return err
	}
	if nodes, err = clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err != nil {
		return err
	}

	if checkerStatus, err = getCheckerStatus(k8sRestClient); err != nil {
		return err
	}

	caData, err = ioutil.ReadFile(config.CAFile)
	if err != nil {
		klog.Errorf("could not find ca file %+v\n", err)
	}
	extraStatus = getExtraStatus(k8sRestClient, config)
	hbData := apistructs.HeartBeatReq{
		Name:           clusterName,
		Address:        config.Host,
		ProbeNamespace: os.Getenv("POD_NAMESPACE"),
		CaData:         base64.StdEncoding.EncodeToString(caData),
		CertData:       base64.StdEncoding.EncodeToString(config.CertData),
		KeyData:        base64.StdEncoding.EncodeToString(config.KeyData),
		Token:          base64.StdEncoding.EncodeToString([]byte(config.BearerToken)),
		Version:        version.String(),
		NodeCount:      len(nodes.Items),
		Checkers:       checkerStatus,
		ExtraStatus:    extraStatus,
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

	d, _ := time.ParseDuration("-4h")
	oneDayAgo := time.Now().Add(d)

	for _, i := range probes.Items {
		if i.Spec.Policy.RunInterval > 0 {
			probeNames = append(probeNames, i.Name)
		}
	}
	for _, i := range probeStatus.Items {
		if IsContain(probeNames, i.Name) {
			for _, j := range i.Spec.Checkers {
				if j.LastRun.Before(&metav1.Time{Time: oneDayAgo}) {
					continue
				}
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

func IsContain(items []string, item string) bool {
	for _, eachItem := range items {
		if eachItem == item {
			return true
		}
	}
	return false
}
