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

package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/remotedialer"
)

var connected = make(chan struct{})

const (
	tokenFile  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	rootCAFile = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

func getClusterInfo(apiserverAddr string) (map[string]interface{}, error) {
	caData, err := ioutil.ReadFile(rootCAFile)
	if err != nil {
		return nil, errors.Wrapf(err, "reading %s", rootCAFile)
	}

	token, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		return nil, errors.Wrapf(err, "reading %s", tokenFile)
	}
	return map[string]interface{}{
		"address": apiserverAddr,
		"token":   strings.TrimSpace(string(token)),
		"caCert":  base64.StdEncoding.EncodeToString(caData),
	}, nil
}

type HeartBeatReq struct {
	Address   string `json:"address"`
	CaData    string `json:"caData"`
	CertData  string `json:"certData"`
	KeyData   string `json:"keyData"`
	Token     string `json:"token"`
	Version   string `json:"version"`
	NodeCount string `json:"nodeCount"`
}

func Start(ctx context.Context, cfg *Config) error {
	//headers := http.Header{
	//	"X-Erda-Cluster-Key": {cfg.ClusterKey},
	//	// TODO: support encode with secretKey
	//	"Authorization": {cfg.ClusterKey},
	//}
	////if cfg.CollectClusterInfo {
	//clusterInfo, err := getClusterInfo(cfg.K8SApiServerAddr)
	//if err != nil {
	//	return err
	//}
	//bytes, err := json.Marshal(clusterInfo)
	//if err != nil {
	//	return err
	//}
	//headers["X-Erda-Cluster-Info"] = []string{base64.StdEncoding.EncodeToString(bytes)}
	//}
	headers := http.Header{
		"X-Erda-Cluster-Key": {"moon"},
		"Authorization":      {"moon"},
	}
	hbData := HeartBeatReq{
		Address:   "XXXXX",
		CaData:    "XXXX",
		CertData:  "XXX",
		KeyData:   "XXX",
		Token:     "XXXX",
		Version:   "XXXXX",
		NodeCount: "XXXXXXXXXX",
	}
	json_data, _ := json.Marshal(hbData)

	go func() {
		for {
			select {
			case <-time.After(5 * time.Second):
				rsp, _ := http.Post(cfg.ClusterHeatBeatEndpoint, "application/json", bytes.NewBuffer(json_data))
				body, _ := ioutil.ReadAll(rsp.Body)
				fmt.Println(string(body))
				rsp.Body.Close()
			}
		}
	}()
	for {
		remotedialer.ClientConnect(ctx, cfg.ClusterDialEndpoint, headers, nil, func(proto, address string) bool {
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
			return nil
		case <-time.After(time.Duration(rand.Int()%10) * time.Second):
			// retry connect after sleep a random time
		}
	}

}

func onConnect(ctx context.Context, session *remotedialer.Session) error {
	connected <- struct{}{}
	return nil

}
