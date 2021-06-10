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
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/rancher/remotedialer"
	"net"
	"net/http"
	"strings"
	"sync"
)

var (
	l       sync.Mutex
	clients = map[string]*http.Client{}
	counter int64
)

type cluster struct {
	Address string `json:"address"`
	Token   string `json:"token"`
	CACert  string `json:"caCert"`
}

func clusterRegister(server *remotedialer.Server, rw http.ResponseWriter, req *http.Request, needClusterInfo bool) {
	//fmt.Println("xxxxxxx")
	//if needClusterInfo {
	//	info := req.Header.Get("X-Erda-Cluster-Info")
	//	fmt.Printf("cluster-info is %+v\n", info)
	//	if info == "" {
	//		remotedialer.DefaultErrorWriter(rw, req, 400, errors.New("missing header:X-Erda-Cluster-Info"))
	//		return
	//	}
	//	var clusterInfo cluster
	//	bytes, err := base64.StdEncoding.DecodeString(info)
	//	if err != nil {
	//		remotedialer.DefaultErrorWriter(rw, req, 400, err)
	//		return
	//	}
	//	if err := json.Unmarshal(bytes, &clusterInfo); err != nil {
	//		remotedialer.DefaultErrorWriter(rw, req, 400, err)
	//		return
	//	}
	//	if clusterInfo.Address == "" {
	//		err = errors.New("invalid cluster info, address empty")
	//		remotedialer.DefaultErrorWriter(rw, req, 400, err)
	//		return
	//	}
	//	if clusterInfo.Token == "" {
	//		err = errors.New("invalid cluster info, token empty")
	//		remotedialer.DefaultErrorWriter(rw, req, 400, err)
	//		return
	//	}
	//	if clusterInfo.CACert == "" {
	//		err = errors.New("invalid cluster info, caCert empty")
	//		remotedialer.DefaultErrorWriter(rw, req, 400, err)
	//		return
	//	}
	//	// TODO: register cluster info
	//}
	server.ServeHTTP(rw, req)
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

func heartbeat(rw http.ResponseWriter, req *http.Request) {
	hbData := HeartBeatReq{}
	if err := json.NewDecoder(req.Body).Decode(&hbData); err != nil {
		rw.WriteHeader(http.StatusBadRequest)

	}
	fmt.Printf("hhhh heartbeat data is %+v\n", hbData)
	rw.WriteHeader(http.StatusOK)
}
func Start(ctx context.Context, cfg *Config) error {
	handler := remotedialer.New(Authorizer, remotedialer.DefaultErrorWriter)
	handler.ClientConnectAuthorizer = func(proto, address string) bool {
		if strings.HasSuffix(proto, "::tcp") {
			return true
		}
		if strings.HasSuffix(proto, "::unix") {
			return address == "/var/run/docker.sock"
		}
		if strings.HasSuffix(proto, "::npipe") {
			return address == "//./pipe/docker_engine"
		}
		return false
	}
	// TODO: support handler.AddPeer
	router := mux.NewRouter()
	router.Handle("/clusterdialer", handler)
	router.HandleFunc("/heartbeat", func(rw http.ResponseWriter,
		req *http.Request) {
		heartbeat(rw, req)
	})
	router.Path("/heartbeat").Methods(http.MethodPost).HandlerFunc(heartbeat)
	router.HandleFunc("/clusteragent/connect", func(rw http.ResponseWriter,
		req *http.Request) {
		clusterRegister(handler, rw, req, cfg.NeedClusterInfo)
	})
	server := &http.Server{
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
		Addr:    cfg.Listen,
		Handler: router,
	}
	return server.ListenAndServe()
}
