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

package dingding

import (
	"context"
	"encoding/json"

	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/pkg/probe-master/k8sclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	_ "github.com/erda-project/kubeprober/pkg/probe-master/k8sclient"
)

func SendAlert(w http.ResponseWriter, r *http.Request, alert *kubeproberv1.Alert) {
	fmt.Printf("xxxxxxx, %+v\n", alert)
	u, _ := url.Parse(alert.Spec.Address)
	fmt.Printf("forwarding to -> %s\n", u)
	proxy := NewProxy(u)
	proxy.Transport = &DebugTransport{}
	if err := alertCount(alert); err != nil {
		klog.Errorf("failed to add alert count: %+v\n", err)
	}
	proxy.ServeHTTP(w, r)
}

func alertCount(al *kubeproberv1.Alert) error {
	var err error
	alert := &kubeproberv1.Alert{}
	if err = k8sclient.RestClient.Get(context.Background(), client.ObjectKey{
		Namespace: al.Namespace,
		Name:      al.Name,
	}, alert); err != nil {
		return err
	}
	now := time.Now()
	nowDay := now.Format("2006-01-02")
	if alert.Status.AlertCount == nil {
		alert.Status.AlertCount = make(map[string]int)
	}
	alert.Status.AlertCount[nowDay]++

	if len(alert.Status.AlertCount) > 30 {
		deleteDay := now.AddDate(0, 0, -30).Format("2006-01-02")
		delete(alert.Status.AlertCount, deleteDay)
	}
	statusPatchBody := kubeproberv1.Alert{
		Status: alert.Status,
	}
	statusPatch, _ := json.Marshal(statusPatchBody)
	err = k8sclient.RestClient.Status().Patch(context.Background(), &kubeproberv1.Alert{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alert.Name,
			Namespace: alert.Namespace,
		},
	}, client.RawPatch(types.MergePatchType, statusPatch))
	if err != nil {
		return err
	}
	return nil
}

type DebugTransport struct{}

func (DebugTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	b, err := httputil.DumpRequestOut(r, false)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(b))
	return http.DefaultTransport.RoundTrip(r)
}

func NewProxy(target *url.URL) *httputil.ReverseProxy {
	targetQuery := target.RawQuery
	director := func(req *http.Request) {
		req.Host = target.Host
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}
		if _, ok := req.Header["User-Agent"]; !ok {
			req.Header.Set("User-Agent", "")
		}
	}
	return &httputil.ReverseProxy{Director: director}
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
