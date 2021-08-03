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

package webserver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logger "sigs.k8s.io/controller-runtime/pkg/log"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/pkg/probe-agent/controllers"
)

type Server struct {
	client          client.Client
	ProbeListenAddr string // the listen address, such as ":80"
}

func NewServer(c client.Client, addr string) Server {
	s := Server{client: c, ProbeListenAddr: addr}
	return s
}

func (s *Server) Start() {
	go func() {
		// Accept status reports coming from external checker pods
		http.HandleFunc("/probe-status", func(w http.ResponseWriter, r *http.Request) {
			err := s.ProbeResultHandler(w, r)
			if err != nil {
				logger.Log.Error(err, "probe-status endpoint error")
			}
		})

		for {
			logger.Log.Info(fmt.Sprintf("starting web server on port: %s", s.ProbeListenAddr))
			err := http.ListenAndServe(s.ProbeListenAddr, nil)
			if err != nil {
				logger.Log.Error(err, "start web server failed", "ProbeListenAddr", s.ProbeListenAddr)
				time.Sleep(time.Second)
			}
		}

	}()
}

func (s Server) Client() client.Client {
	return s.client
}

func (s *Server) ProbeResultHandler(w http.ResponseWriter, r *http.Request) error {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		logger.Log.Error(err, "read request body failed", "body", string(b))
		return nil
	}

	rp := kubeproberv1.ReportProbeStatusSpec{}
	err = json.Unmarshal(b, &rp)
	for i := range rp.Checkers {
		s := strings.ToUpper(string(rp.Checkers[i].Status))
		rp.Checkers[i].Status = kubeproberv1.CheckerStatus(s)
	}
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		logger.Log.Error(err, "unmarshal request body failed", "body", string(b))
		return nil
	}

	err = controllers.ReportProbeResult(s.client, rp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Log.Error(err, "process probe item status failed", "probe item status", rp)
		return nil
	}

	w.WriteHeader(http.StatusOK)
	logger.Log.Info(fmt.Sprintf("process probe item status successfully, key: %s/%s/%s", rp.ProbeNamespace, rp.ProbeName, rp.Name))
	return nil
}
