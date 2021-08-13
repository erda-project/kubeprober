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

package apistructs

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// HeartBeatReq heatbeat request struct between probe-master and probe-agent
type HeartBeatReq struct {
	Name           string `json:"name"`
	SecretKey      string `json:"secretKey"`
	Address        string `json:"address"`
	CaData         string `json:"caData"`
	CertData       string `json:"certData"`
	KeyData        string `json:"keyData"`
	Token          string `json:"token"`
	Version        string `json:"version"`
	NodeCount      int    `json:"nodeCount"`
	ProbeNamespace string `json:"probeNamespace"`
	Checkers       string `json:"checkers"`
}

type CollectProbeStatusReq struct {
	ClusterName string       `json:"clusterName"`
	ProbeName   string       `json:"probeName"`
	CheckerName string       `json:"checkerName"`
	Status      string       `json:"status"`
	Message     string       `json:"message"`
	LastRun     *metav1.Time `json:"lastRun,omitempty"`
}
