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

package probe_status

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/cenkalti/backoff"

	kubeprobev1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/sirupsen/logrus"
)

const (
	maxReportTime = time.Second * 30
)

func ReportProbeStatus(status []kubeprobev1.ProbeCheckerStatus) error {
	MOCK := os.Getenv("USE_MOCK")
	if MOCK == "true" {
		logrus.Infof("MOCK MODE, probe status: %v", status)
		return nil
	}

	info := ProbeStatusReportInfo{}
	err := info.Init()
	if err != nil {
		logrus.Errorf("init probe status report info failed, error:%v", err)
		return err
	}

	err = ValidateProbeStatus(status)
	if err != nil {
		logrus.Errorf("validate checker status failed, content: %+v, error: %v", status, err)
	}

	pss, err := renderProbeStatus(status, info)
	if err != nil {
		logrus.Errorf("render checker status failed, content: %+v, error: %v", status, err)
		return err
	}

	err = sendProbeStatus(*pss, info)
	if err != nil {
		logrus.Errorf("send probe status failed, error:%v", err)
		return err
	}

	return nil
}

func ValidateProbeStatus(status []kubeprobev1.ProbeCheckerStatus) (err error) {
	for _, s := range status {
		err = s.Validate()
		if err != nil {
			return
		}
	}
	return
}

func sendProbeStatus(ps kubeprobev1.ReportProbeStatusSpec, info ProbeStatusReportInfo) error {
	b, err := json.Marshal(ps)
	if err != nil {
		logrus.Errorf("marshal probe status failed, content:%v, error:%v", ps, err)
		return err
	}

	req, err := http.NewRequest(http.MethodPost, info.ProbeStatusReportUrl, bytes.NewBuffer(b))
	if err != nil {
		logrus.Errorf("new request failed, error:%v", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	bf := backoff.NewExponentialBackOff()
	bf.MaxElapsedTime = maxReportTime

	client := &http.Client{}
	var resp *http.Response
	err = backoff.Retry(func() error {
		var err error
		resp, err = client.Do(req)
		if err != nil {
			return err
		}
		// retry on status codes that do not return a 200 or 400
		if !(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest) {
			err := fmt.Errorf("bad response status code from report url, status:%v, report url:%s", resp.StatusCode, info.ProbeStatusReportUrl)
			logrus.Errorf(err.Error())
			return err
		}
		return nil
	}, bf)
	if err != nil {
		logrus.Errorf("send probe status failed, error:%v", err)
		return err
	}
	logrus.Infof("send probe status successfully, status: %+v", ps)
	return nil
}

func renderProbeStatus(status []kubeprobev1.ProbeCheckerStatus, info ProbeStatusReportInfo) (*kubeprobev1.ReportProbeStatusSpec, error) {
	if len(status) == 0 {
		err := fmt.Errorf("empty report status")
		logrus.Errorf(err.Error())
		return nil, err
	}

	rp := kubeprobev1.ReportProbeStatusSpec{
		ProbeName:      info.ProbeName,
		ProbeNamespace: info.ProbeNamespace,
		Checkers:       status,
	}

	return &rp, nil
}

type ProbeStatusReportInfo struct {
	ProbeNamespace       string
	ProbeName            string
	ProbeStatusReportUrl string
}

func (p *ProbeStatusReportInfo) Init() error {
	err := p.InitProbeNamespace()
	if err != nil {
		return err
	}
	err = p.InitProbeName()
	if err != nil {
		return err
	}
	err = p.InitProbeStatusReportUrl()
	if err != nil {
		return err
	}
	return nil
}

func (p *ProbeStatusReportInfo) InitProbeNamespace() error {
	MOCK := os.Getenv("USE_MOCK")
	if MOCK == "true" {
		logrus.Infof("MOCK MODE, probe namespace: %v", "probe-namespace-mock")
		p.ProbeNamespace = "probe-namespace-mock"
		return nil
	}

	var namespace string
	data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		logrus.Warnf("failed to open namespace file, error:%v", err.Error())
	}

	if len(data) != 0 {
		namespace = string(data)
	} else {
		namespace = os.Getenv(kubeprobev1.ProbeNamespace)
	}

	if namespace == "" {
		err := fmt.Errorf("cannot get probe namespace from secret file or environment")
		logrus.Errorf(err.Error())
		return err

	}

	p.ProbeNamespace = namespace

	return nil
}

func (p *ProbeStatusReportInfo) InitProbeName() error {
	MOCK := os.Getenv("USE_MOCK")
	if MOCK == "true" {
		logrus.Infof("MOCK MODE, probe name: %v", "probe-name-mock")
		p.ProbeNamespace = "probe-name-mock"
		return nil
	}

	name := os.Getenv(kubeprobev1.ProbeName)
	if name == "" {
		err := fmt.Errorf("cannot get probe name from environment")
		logrus.Errorf(err.Error())
		return err
	}
	p.ProbeName = name
	return nil
}

func (p *ProbeStatusReportInfo) InitProbeStatusReportUrl() error {
	MOCK := os.Getenv("USE_MOCK")
	if MOCK == "true" {
		logrus.Infof("MOCK MODE, probe status report url: %v", "http://probe-status-report/mock")
		p.ProbeStatusReportUrl = "http://probeagent.probe-namespace-mock.svc.cluster.local:8082/probe-status"
		return nil
	}

	u := os.Getenv(kubeprobev1.ProbeStatusReportUrl)
	if u == "" {
		err := fmt.Errorf("cannot get probe status report url from environment")
		logrus.Errorf(err.Error())
		return err
	}
	_, err := url.ParseRequestURI(u)
	if err != nil {
		logrus.Errorf("parse url failed, url:%s,  error:%v", u, err)
		return err
	}
	p.ProbeStatusReportUrl = u
	return nil
}
