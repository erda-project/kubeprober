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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/klog"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/cli/probe/tunnel-client/clusterdialer"
)

type ManageConfig struct {
	Type     string `json:"type"`
	Address  string `json:"address"`
	CaData   string `json:"caData"`
	CertData string `json:"certData"`
	KeyData  string `json:"keyData"`
	Token    string `json:"token"`
}

type KpConfig struct {
	MasterAddr string `json:"masterAddr"`
}

const (
	ManageProxy  = "proxy"
	KpConfigFlie = ".kubeprober/config"
)

var MasterAddr string

// get master-addr by config file
func init() {
	filePath := fmt.Sprintf("%s/%s", os.Getenv("HOME"), KpConfigFlie)
	MasterAddr, _ = LoadConfig(filePath)
}

func LoadConfig(path string) (string, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	mainConfig := &KpConfig{}
	err = json.Unmarshal(buf, mainConfig)
	if err != nil {
		return "", err
	}

	return mainConfig.MasterAddr, nil
}

// GetRestConfig get rest.Config with manage config
func GetRestConfig(c *ManageConfig) (*rest.Config, error) {
	// If not provide api-server address, use in-cluster rest config
	if c.Address == "" {
		return rest.InClusterConfig()
	}

	rc := &rest.Config{
		Host:    c.Address,
		APIPath: "/apis",
		QPS:     1000,
		Burst:   100,
		ContentConfig: rest.ContentConfig{
			NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		},
		TLSClientConfig: rest.TLSClientConfig{},
		UserAgent:       rest.DefaultKubernetesUserAgent(),
		RateLimiter:     flowcontrol.NewTokenBucketRateLimiter(1000, 100),
	}

	// If ca data is empty, the certificate is not validated
	if c.CaData == "" {
		rc.TLSClientConfig.Insecure = true
	} else {
		caBytes, err := base64.StdEncoding.DecodeString(c.CaData)
		if err != nil {
			return nil, err
		}

		rc.TLSClientConfig.CAData = caBytes
	}

	// If token is not empty, use token firstly, else use certificate
	if c.Token != "" {
		rc.BearerToken = c.Token
	} else {
		if c.CertData == "" || c.KeyData == "" {
			return nil, fmt.Errorf("must provide available cert data and key data")
		}

		certBytes, err := base64.StdEncoding.DecodeString(c.CertData)
		if err != nil {
			return nil, err
		}

		keyBytes, err := base64.StdEncoding.DecodeString(c.KeyData)
		if err != nil {
			return nil, err
		}

		rc.TLSClientConfig.CertData = certBytes
		rc.TLSClientConfig.KeyData = keyBytes
	}

	return rc, nil
}

var once sync.Once

func Init() {
	if MasterAddr == "" {
		MasterAddr = "ws://probe-master.kubeprober.svc.cluster.local:8088"
	}
	clusterdialer.InitSession(MasterAddr)
}

func GetDialerRestConfig(clusterName string, c *ManageConfig) (*rest.Config, error) {
	once.Do(func() { Init() })

	rc, err := GetRestConfig(c)
	if err != nil {
		return nil, err
	}

	rc.TLSClientConfig.NextProtos = []string{"http/1.1"}
	rc.UserAgent = rest.DefaultKubernetesUserAgent() + " cluster " + clusterName
	rc.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		if ht, ok := rt.(*http.Transport); ok {
			ht.DialContext = clusterdialer.DialContext(clusterName)
		}
		return rt
	}

	return rc, nil
}

func GenerateProbeClientConf(cluster *kubeproberv1.Cluster) (*rest.Config, error) {
	var err error
	var clusterToken []byte
	var config *rest.Config

	if cluster.Spec.ClusterConfig.Token != "" {
		if clusterToken, err = base64.StdEncoding.DecodeString(cluster.Spec.ClusterConfig.Token); err != nil {
			klog.Errorf("token, %+v\n", err)
			return nil, err
		}
		config, err = GetDialerRestConfig(cluster.Name, &ManageConfig{
			Type:    ManageProxy,
			Address: cluster.Spec.ClusterConfig.Address,
			Token:   strings.Trim(string(clusterToken), "\n"),
			CaData:  cluster.Spec.ClusterConfig.CACert,
		})
		if err != nil {
			klog.Errorf("failed to generate dialer rest config for cluster %s, %+v\n", err, cluster.Name)
			return nil, err
		}
	} else {
		config, err = GetDialerRestConfig(cluster.Name, &ManageConfig{
			Type:     ManageProxy,
			Address:  cluster.Spec.ClusterConfig.Address,
			CertData: cluster.Spec.ClusterConfig.CertData,
			KeyData:  cluster.Spec.ClusterConfig.KeyData,
			CaData:   cluster.Spec.ClusterConfig.CACert,
		})
		if err != nil {
			klog.Errorf("failed to generate dialer rest config for cluster %s, %+v\n", err, cluster.Name)
			return nil, err
		}
	}

	return config, err
}
