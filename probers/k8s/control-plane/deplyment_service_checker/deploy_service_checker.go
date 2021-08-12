package deployment_service_checker

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/pkg/kubeclient"
)

const (
	defaultCheckerKey   = "checker"
	defaultCheckerValue = "deployment-service-checker"
)

// Checker validates that deployment is functioning correctly
type DeployServiceChecker struct {
	client  *kubernetes.Clientset
	Name    string
	Status  kubeproberv1.CheckerStatus
	Timeout time.Duration
}

// New returns a new DNS Checker
func NewChecker() (*DeployServiceChecker, error) {
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
func (dc *DeployServiceChecker) DoCheck() (err error) {
	ctx := context.Background()

	defer func() {
		// clean resource
		err = deleteNamespace(ctx, dc.client)
		if err != nil {
			logrus.Errorf("clean resource, delete namespace failed, namespace: %s, error: %v", cfg.CheckNamespace, err)
			return
		}
	}()

	// create deployment
	err = createDeployment(ctx, dc.client)
	if err != nil {
		logrus.Errorf("create deployment failed, error: %v", err)
		return err
	}

	// create service
	err = createService(ctx, dc.client)
	if err != nil {
		logrus.Errorf("create service failed, error: %v", err)
		return err
	}

	// check service
	err = makeRequestToDeploymentCheckService(ctx, dc.client)
	if err != nil {
		logrus.Errorf("request to service failed, error: %v", err)
		return err
	}

	return nil
}
