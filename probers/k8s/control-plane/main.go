package main

import (
	"fmt"

	"github.com/sirupsen/logrus"

	proberchecker "github.com/erda-project/kubeprober/pkg/probe-checker"
	"github.com/erda-project/kubeprober/probers/k8s/control-plane/config"
	svc "github.com/erda-project/kubeprober/probers/k8s/control-plane/deplyment_service_checker"
	dns "github.com/erda-project/kubeprober/probers/k8s/control-plane/dns_resolution_checker"
	ns "github.com/erda-project/kubeprober/probers/k8s/control-plane/namespace-checker"
)

func main() {
	var (
		err error
		s   *svc.DeployServiceChecker
		d   *dns.DnsChecker
		n   *ns.NamespaceChecker
	)

	defer func() {
		if err != nil {
			panic(err)
		}
	}()

	// load config
	config.Load()
	err = config.ParseConfig()
	if err != nil {
		err = fmt.Errorf("parse config failed, error: %v", err)
		return
	}
	// check log debug level
	if config.Cfg.Debug {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debug("DEBUG MODE")
	}

	// create checkers
	// deployment service checker
	s, err = svc.NewChecker()
	if err != nil {
		err = fmt.Errorf("new deployment service checker failed, error: %v", err)
		return
	}

	// dns checker
	d, err = dns.NewChecker()
	if err != nil {
		err = fmt.Errorf("new dns checker failed, error: %v", err)
		return
	}

	n, err = ns.NewChecker()
	if err != nil {
		err = fmt.Errorf("new namespace checker failed, error: %v", err)
		return
	}

	// run checkers
	err = proberchecker.RunCheckers(proberchecker.CheckerList{s, d, n})
	if err != nil {
		err = fmt.Errorf("run deployment service checker failed, error: %v", err)
		return
	}
	logrus.Infof("run deployment service checker successfully")
}
