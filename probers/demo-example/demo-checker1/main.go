package main

import (
	"github.com/sirupsen/logrus"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	probestatus "github.com/erda-project/kubeprober/pkg/probe-status"
)

func main() {
	// checker1 item1
	// do real check ..., and get check status
	item1 := kubeproberv1.ProbeCheckerStatus{
		Name:    "checker1 item1",
		Status:  kubeproberv1.CheckerStatusPass,
		Message: "",
	}

	// checker1 item2
	// do real check ..., and get check status
	item2 := kubeproberv1.ProbeCheckerStatus{
		Name:    "checker1 item2",
		Status:  kubeproberv1.CheckerStatusError,
		Message: "do check item2 failed, reason: ...",
	}

	err := probestatus.ReportProbeStatus([]kubeproberv1.ProbeCheckerStatus{item1, item2})
	if err != nil {
		logrus.Errorf("report probe status failed, error: %v", err)
	}
}
