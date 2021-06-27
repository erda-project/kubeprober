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
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	probev1 "github.com/erda-project/kubeprober/pkg/probe-agent/apis/v1"
)

const (
	maxReportTime = time.Second * 30
)

type ReportProbeStatusSpec struct {
	ProbeName               string `json:"probeName"`
	ProbeNamespace          string `json:"probeNamespace"`
	probev1.ProbeItemStatus `json:",inline"`
}

func ReportProbeStatus(status []probev1.ProbeCheckerStatus) error {
	info := ProbeStatusReportInfo{}
	err := info.Init()
	if err != nil {
		logrus.Errorf("init probe status report info failed, error:%v", err)
		return err
	}

	pss, err := renderProbeStatus(status, info)
	if err != nil {
		logrus.Errorf("render checker status failed, content:%v, error:%v", status, err)
		return err
	}

	err = sendProbeStatus(*pss, info)
	if err != nil {
		return err
	}

	return nil
}

func sendProbeStatus(ps ReportProbeStatusSpec, info ProbeStatusReportInfo) error {
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
	return nil
}

func renderProbeStatus(status []probev1.ProbeCheckerStatus, info ProbeStatusReportInfo) (*ReportProbeStatusSpec, error) {
	if len(status) == 0 {
		err := fmt.Errorf("empty report status")
		logrus.Errorf(err.Error())
		return nil, err
	}

	now := metav1.Now()
	pcs := probev1.ProbeCheckerStatus{
		Status:  probev1.CheckerStatusInfo,
		LastRun: &now,
	}

	for i, s := range status {
		err := s.Validate()
		if err != nil {
			logrus.Errorf("status validate failed, content:%v, error:%v", s, err)
			return nil, err
		}
		if s.LastRun == nil {
			status[i].LastRun = &now
		}
		if s.Status == probev1.CheckerStatusUNKNOWN {
			logrus.Warnf("probe checker status should not be UNKNOWN")
			continue
		}
		if pcs.Status != probev1.CheckerStatusError && s.Status != probev1.CheckerStatusInfo {
			pcs.Status = s.Status
		}
	}

	pis := probev1.ProbeItemStatus{
		ProbeCheckerStatus: probev1.ProbeCheckerStatus{
			Name:    info.ProbeItemName,
			Status:  pcs.Status,
			LastRun: pcs.LastRun,
		},
		Checkers: status,
	}

	rp := ReportProbeStatusSpec{
		ProbeName:       info.ProbeName,
		ProbeNamespace:  info.ProbeNamespace,
		ProbeItemStatus: pis,
	}

	return &rp, nil
}

type ProbeStatusReportInfo struct {
	ProbeNamespace       string
	ProbeName            string
	ProbeItemName        string
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
	err = p.InitProbeItemName()
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

	var namespace string

	data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		logrus.Warnf("failed to open namespace file, error:%v", err.Error())
	}

	if len(data) != 0 {
		namespace = string(data)
	} else {
		namespace = os.Getenv(probev1.ProbeNamespace)
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
	name := os.Getenv(probev1.ProbeName)
	if name == "" {
		err := fmt.Errorf("cannot get probe name from environment")
		logrus.Errorf(err.Error())
		return err
	}
	p.ProbeName = name
	return nil
}

func (p *ProbeStatusReportInfo) InitProbeItemName() error {
	name := os.Getenv(probev1.ProbeItemName)
	if name == "" {
		err := fmt.Errorf("cannot get probe item name from environment")
		logrus.Errorf(err.Error())
		return err
	}
	p.ProbeItemName = name
	return nil
}

func (p *ProbeStatusReportInfo) InitProbeStatusReportUrl() error {
	u := os.Getenv(probev1.ProbeStatusReportUrl)
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
