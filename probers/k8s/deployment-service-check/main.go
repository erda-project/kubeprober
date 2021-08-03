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
		dc  *DeployServiceChecker
	)

	defer func() {
		if err != nil {
			panic(err)
		}
	}()

	// load config
	ConfigLoad()
	err = ParseConfig()
	if err != nil {
		err = fmt.Errorf("parse config failed, error: %v", err)
		return
	}
	// check log debug level
	if cfg.Debug {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debug("DEBUG MODE")
	}

	// create checkers
	// dns checker
	dc, err = NewDeployServiceChecker()
	if err != nil {
		err = fmt.Errorf("new dns checker failed, error: %v", err)
		return
	}

	// run checkers
	err = proberchecker.RunCheckers(proberchecker.CheckerList{dc})
	if err != nil {
		err = fmt.Errorf("run deployment service checker failed, error: %v", err)
		return
	}
	logrus.Infof("run deployment service checker successfully")
}
