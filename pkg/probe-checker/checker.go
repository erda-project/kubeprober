package v1

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	probestatus "github.com/erda-project/kubeprober/pkg/probe-status"
)

type Checker interface {
	GetName() string
	SetName(string)
	GetStatus() kubeproberv1.CheckerStatus
	SetStatus(kubeproberv1.CheckerStatus)
	GetTimeout() time.Duration
	SetTimeout(time.Duration)
	DoCheck() error
}

type CheckerList []Checker

func RunCheckers(cs CheckerList) error {
	var sg sync.WaitGroup
	ss := make([]kubeproberv1.ProbeCheckerStatus, 0)
	sg.Add(len(cs))
	for _, c := range cs {
		go func(cr Checker) {
			defer func() {
				sg.Done()
			}()
			s := kubeproberv1.ProbeCheckerStatus{
				Name:   cr.GetName(),
				Status: kubeproberv1.CheckerStatusPass,
			}
			err := RunChecker(cr)
			if err != nil {
				if cr.GetStatus() != kubeproberv1.CheckerStatusPass || cr.GetStatus() != kubeproberv1.CheckerStatusWARN {
					s.Status = kubeproberv1.CheckerStatusError
				}
				s.Message = err.Error()
			}
			now := metav1.Now()
			s.LastRun = &now
			ss = append(ss, s)
		}(c)
	}
	sg.Wait()
	err := probestatus.ReportProbeStatus(ss)
	if err != nil {
		logrus.Errorf("report probe status failed, error: %v", err)
		return err
	}
	return nil
}

func RunChecker(c Checker) error {
	logrus.Infof("start checker: %s", c.GetName())

	// run the check in a goroutine and notify the doneChan when completed
	doneChan := make(chan error)
	go func(doneChan chan error) {
		err := c.DoCheck()
		doneChan <- err
	}(doneChan)

	// if timeout not set, given 10 min
	if c.GetTimeout() < 200*time.Millisecond {
		c.SetTimeout(10 * time.Minute)
	}

	// wait for either a timeout or job completion
	select {
	case <-time.After(c.GetTimeout()):
		// The check has timed out after its specified timeout period
		err := fmt.Errorf("checker: %s timeout: %v", c.GetName(), c.GetTimeout())
		logrus.Errorf(err.Error())
		return err
	case err := <-doneChan:
		if err != nil {
			logrus.Errorf(err.Error())
			return err
		}
		return nil
	}
}
