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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/apistructs"
	"github.com/gorilla/mux"
	"github.com/rancher/remotedialer"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func clusterRegister(server *remotedialer.Server, rw http.ResponseWriter, req *http.Request) {
	server.ServeHTTP(rw, req)
}

func heartbeat(rw http.ResponseWriter, req *http.Request) {
	hbData := apistructs.HeartBeatReq{}
	cluster := &kubeproberv1.Cluster{}
	var err error

	if err := json.NewDecoder(req.Body).Decode(&hbData); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	clusterSpec := kubeproberv1.Cluster{
		Spec: kubeproberv1.ClusterSpec{
			K8sVersion: hbData.Version,
			ClusterConfig: kubeproberv1.ClusterConfig{
				Address:         hbData.Address,
				Token:           hbData.Token,
				CACert:          hbData.CaData,
				CertData:        hbData.CertData,
				KeyData:         hbData.KeyData,
				ProbeNamespaces: hbData.ProbeNamespace,
			},
		},
	}
	err = k8sRestClient.Get(context.Background(), client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      hbData.Name,
	}, cluster)

	if apierrors.IsNotFound(err) {
		clusterSpec.ObjectMeta = metav1.ObjectMeta{
			Name:      hbData.Name,
			Namespace: metav1.NamespaceDefault,
		}
		if err = k8sRestClient.Create(context.Background(), &clusterSpec); err != nil {
			errMsg := fmt.Sprintf("[heartbeat] failed to create cluster [%s]: %+v\n", hbData.Name, err)
			rw.Write([]byte(errMsg))
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
	} else {
		patch, _ := json.Marshal(clusterSpec)
		err = k8sRestClient.Patch(context.Background(), &kubeproberv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hbData.Name,
				Namespace: metav1.NamespaceDefault,
			},
		}, client.RawPatch(types.MergePatchType, patch))
		if err != nil {
			errMsg := fmt.Sprintf("[heartbeat] patch cluster[%s] spec error: %+v\n", hbData.Name, err)
			klog.Errorf(errMsg)
			rw.Write([]byte(errMsg))
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	statusPatchBody := kubeproberv1.Cluster{
		Status: kubeproberv1.ClusterStatus{
			HeartBeatTimeStamp: time.Now().Format("2006-01-02 15:04:05"),
			NodeCount:          hbData.NodeCount,
		},
	}
	statusPatch, _ := json.Marshal(statusPatchBody)
	err = k8sRestClient.Status().Patch(context.Background(), &kubeproberv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hbData.Name,
			Namespace: metav1.NamespaceDefault,
		},
	}, client.RawPatch(types.MergePatchType, statusPatch))
	if err != nil {
		errMsg := fmt.Sprintf("[heartbeat] patch cluster[%s] status error: %+v\n", hbData.Name, err)
		klog.Errorf(errMsg)
		rw.Write([]byte(errMsg))
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	rw.WriteHeader(http.StatusOK)
	return
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
	router.Path("/heartbeat").Methods(http.MethodPost).HandlerFunc(heartbeat)
	router.HandleFunc("/clusteragent/connect", func(rw http.ResponseWriter,
		req *http.Request) {
		clusterRegister(handler, rw, req)
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
