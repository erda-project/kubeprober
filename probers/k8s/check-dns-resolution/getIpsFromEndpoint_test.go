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

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kuberhealthy/kuberhealthy/v2/pkg/kubeClient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetIpsFromEndpoint(t *testing.T) {
	client, err := kubeClient.Create(filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	if err != nil {
		t.Fatalf("Unable to create kube client")
	}
	endpoints, err := client.CoreV1().Endpoints("kube-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "k8s-app=kube-dns"})
	if err != nil {
		t.Fatalf("Unable to get endpoint list")
	}

	ips, err := getIpsFromEndpoint(endpoints)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if len(ips) < 1 {
		t.Fatalf("No ips found from endpoint list")
	}

}
