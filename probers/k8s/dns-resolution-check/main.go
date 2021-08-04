package main

import (
	"fmt"

	"github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	proberchecker "github.com/erda-project/kubeprober/pkg/probe-checker"
)

func main() {
	var (
		err error
		dc  *DnsChecker
	)

	defer func() {
		if err != nil {
			panic(err)
		}
	}()

	// load config
	ConfigLoad()

	// check log debug level
	if cfg.Debug {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debug("DEBUG MODE")
	}

	// create checkers
	// dns checker
	dc, err = NewDnsChecker()
	if err != nil {
		err = fmt.Errorf("new dns checker failed, error: %v", err)
		return
	}

	// run checkers
	err = proberchecker.RunCheckers(proberchecker.CheckerList{dc})
	if err != nil {
		err = fmt.Errorf("run dns checker failed, private domain: %s, public domain: %s, error: %v", cfg.PrivateDomain, cfg.PublicDomain, err)
		return
	}
	logrus.Infof("run dns check success for for private domain: %s, public domain: %s", cfg.PrivateDomain, cfg.PublicDomain)
}
