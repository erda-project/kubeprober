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

package app

import (
	"encoding/base64"
	"strings"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	dialclient "github.com/erda-project/kubeprober/cli/probe/tunnel-client"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

//Generate k8sclient of cluster
func GenerateProbeClient(cluster *kubeproberv1.Cluster) (client.Client, error) {
	var clusterToken []byte
	var err error
	var c client.Client
	var config *rest.Config

	if cluster.Spec.ClusterConfig.Token != "" {
		if clusterToken, err = base64.StdEncoding.DecodeString(cluster.Spec.ClusterConfig.Token); err != nil {
			klog.Errorf("token, %+v\n", err)
			return nil, err
		}
		config, err = dialclient.GetDialerRestConfig(cluster.Name, &dialclient.ManageConfig{
			Type:    dialclient.ManageProxy,
			Address: cluster.Spec.ClusterConfig.Address,
			Token:   strings.Trim(string(clusterToken), "\n"),
			CaData:  cluster.Spec.ClusterConfig.CACert,
		})
		if err != nil {
			return nil, err
		}
	} else {
		config, err = dialclient.GetDialerRestConfig(cluster.Name, &dialclient.ManageConfig{
			Type:     dialclient.ManageProxy,
			Address:  cluster.Spec.ClusterConfig.Address,
			CertData: cluster.Spec.ClusterConfig.CertData,
			KeyData:  cluster.Spec.ClusterConfig.KeyData,
			CaData:   cluster.Spec.ClusterConfig.CACert,
		})
		if err != nil {
			return nil, err
		}
	}
	scheme := runtime.NewScheme()
	kubeproberv1.AddToScheme(scheme)
	clientgoscheme.AddToScheme(scheme)

	c, err = client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}
	return c, nil
}
