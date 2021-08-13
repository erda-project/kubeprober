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
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/apistructs"
	"github.com/erda-project/kubeprober/pkg/probe-master/k8sclient"
	_ "github.com/erda-project/kubeprober/pkg/probe-master/k8sclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const DINGDING_ALERT_NAME = "dingding"

var ci = make(chan int, 100)

func init() {
	var count = 0
	var err error
	go func() {
		for {
			select {
			case <-ci:
				count++
			case <-time.After(60 * time.Second):
				if err = alertCount(count); err != nil {
					klog.Errorf("failed to count alert number: %+v\n", err)
				}
				count = 0
			}
		}
	}()
}

func ProxyAlert(w http.ResponseWriter, r *http.Request, alert *kubeproberv1.Alert) {
	u, _ := url.Parse(alert.Spec.Address)
	fmt.Printf("forwarding to -> %s\n", u)
	proxy := NewProxy(u)
	proxy.Transport = &DebugTransport{}
	ci <- 1
	proxy.ServeHTTP(w, r)
}

func alertCount(count int) error {
	var err error
	alert := &kubeproberv1.Alert{}
	if err = k8sclient.RestClient.Get(context.Background(), client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      DINGDING_ALERT_NAME,
	}, alert); err != nil {
		return err
	}
	now := time.Now()
	loc, _:= time.LoadLocation("Asia/Shanghai")
	nowDay := now.In(loc).Format("2006-01-02")
	if alert.Status.AlertCount == nil {
		alert.Status.AlertCount = make(map[string]int)
	}
	alert.Status.AlertCount[nowDay] = alert.Status.AlertCount[nowDay] + count

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
			Name:      DINGDING_ALERT_NAME,
			Namespace: metav1.NamespaceDefault,
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

func SendAlert(ps *apistructs.CollectProbeStatusReq, alert *kubeproberv1.Alert) error {
	var err error
	var signUrl string
	var resp *http.Response
	var result []byte
	if alert.Spec.Address == "" || alert.Spec.Token == "" || alert.Spec.Sign == "" {
		return errors.New("address or token or sign is emtpy in this alert spec")
	}
	addr := fmt.Sprintf("%s/robot/send?access_token=%s", alert.Spec.Address, alert.Spec.Token)
	if signUrl, err = getSignURL(addr, alert.Spec.Sign); err != nil {
		return err
	}

	istr := "[类别]: " + ps.ProbeName + "\n" +
		"[检查项]：" + ps.CheckerName + "\n" +
		"[集群]：" + ps.ClusterName + "\n" +
		"[状态]: " + ps.Status + "\n" +
		"[错误信息]: " + ps.Message

	content, data := make(map[string]string), make(map[string]interface{})
	content["content"] = istr
	data["msgtype"] = "text"
	data["text"] = content
	b, _ := json.Marshal(data)

	body := bytes.NewBuffer(b)

	resp, err = http.Post(signUrl, "application/json;charset=utf-8", body)
	if err != nil {
		klog.Errorf("faile to send msg to dingding: %+v\n", err)
		return err
	}
	result, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		klog.Errorf("faile to send msg to dingding: %+v\n", err)
		return err
	}
	klog.Infof("dingding return msg: %s\n", result)
	return nil
}

func getSignURL(addr string, sign string) (string, error) {
	if sign == "" {
		return "", nil
	}

	tm := time.Now().UnixNano() / 1e6
	strToHash := fmt.Sprintf("%d\n%s", tm, sign)
	hmac256 := hmac.New(sha256.New, []byte(sign))
	hmac256.Write([]byte(strToHash))
	data := hmac256.Sum(nil)
	dataStr := base64.StdEncoding.EncodeToString(data)

	u, err := url.Parse(addr)
	if err != nil {
		return "", err
	}
	values := u.Query()
	values.Set("timestamp", fmt.Sprintf("%d", tm))
	values.Set("sign", dataStr)
	u.RawQuery = values.Encode()

	return u.String(), nil
}
