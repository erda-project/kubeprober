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
	"fmt"

	"github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

//const maxTimeInFailure = 60 * time.Second
//const defaultCheckTimeout = 5 * time.Minute
//
//// KubeConfigFile is a variable containing file path of Kubernetes config files
//var KubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
//
//// CheckTimeout is a variable for how long code should run before it should retry
//var CheckTimeout time.Duration
//
//// Hostname is a variable for container/pod name
//var Hostname string
//
//// NodeName is a variable for the node where the container/pod is created
//var NodeName string
//
//// Namespace where dns pods live
//var namespace string
//
//// Label selector used for dns pods
//var labelSelector string
//
//var cfg.PublicDomain string
//var now time.Time
//
//func init() {
//
//	// Set check time limit to default
//	CheckTimeout = defaultCheckTimeout
//
//	Hostname = os.Getenv("HOSTNAME")
//	if len(Hostname) == 0 {
//		logrus.Errorln("ERROR: The ENDPOINT environment variable has not been set.")
//		return
//	}
//
//	NodeName = os.Getenv("NODE_NAME")
//	if len(NodeName) == 0 {
//		logrus.Errorln("ERROR: Failed to retrieve NODE_NAME environment variable.")
//		return
//	}
//	logrus.Infoln("Check pod is running on node:", NodeName)
//
//	namespace = os.Getenv("NAMESPACE")
//	if len(namespace) > 0 {
//		logrus.Infoln("Looking for DNS pods in namespace:", namespace)
//	}
//
//	labelSelector = os.Getenv("DNS_POD_SELECTOR")
//	if len(labelSelector) > 0 {
//		logrus.Infoln("Looking for DNS pods with label:", labelSelector)
//	}
//	cfg.PublicDomain = os.Getenv("TEST_PUBLIC_DOMAIN")
//	now = time.Now()
//}

func main() {
	var (
		err error
		c   *Checker
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

	// new & run checker
	c, err = NewChecker()
	if err != nil {
		err = fmt.Errorf("new checker failed, error: %v", err)
		return
	}
	err = c.Run()
	if err != nil {
		err = fmt.Errorf("run dns check failed for hostname: %s, error: %v", cfg.Hostname, err)
		return
	}
	logrus.Infoln("run dns check success for hostname:", cfg.Hostname)
}
