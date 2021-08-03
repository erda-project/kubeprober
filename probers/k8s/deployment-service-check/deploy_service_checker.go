package main

import (
	"context"
	"time"

	"github.com/erda-project/kubeprober/pkg/kubeclient"
	"github.com/sirupsen/logrus"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
)

// New returns a new DNS Checker
func NewDeployServiceChecker() (*DeployServiceChecker, error) {
	// get kubernetes client
	client, err := kubeclient.Client(cfg.KubeConfigFile)
	if err != nil {
		logrus.Fatalln("Unable to create kubernetes client", err)
		return nil, err
	}
	return &DeployServiceChecker{
		client:  client,
		Name:    "deployment-service-check",
		Timeout: cfg.CheckTimeout,
	}, nil
}

func (dc *DeployServiceChecker) GetName() string {
	return dc.Name
}

func (dc *DeployServiceChecker) SetName(n string) {
	dc.Name = n
}

func (dc *DeployServiceChecker) GetStatus() kubeproberv1.CheckerStatus {
	return dc.Status
}

func (dc *DeployServiceChecker) SetStatus(s kubeproberv1.CheckerStatus) {
	dc.Status = s
}

func (dc *DeployServiceChecker) GetTimeout() time.Duration {
	return dc.Timeout
}

func (dc *DeployServiceChecker) SetTimeout(t time.Duration) {
	dc.Timeout = t
}

// doChecks does validations on the DNS call to the endpoint
func (dc *DeployServiceChecker) DoCheck() error {
	ctx := context.Background()
	err := createDeployment(ctx, dc.client)
	if err != nil {
		logrus.Errorf("create deployment failed, error: %v", err)
		return err
	}
	err = createService(ctx, dc.client, nil)
	if err != nil {
		logrus.Errorf("create service failed, error: %v", err)
		return err
	}
	err = makeRequestToDeploymentCheckService(ctx, dc.client)
	if err != nil {
		logrus.Errorf("request to service failed, error: %v", err)
		return err
	}
	return nil
}
