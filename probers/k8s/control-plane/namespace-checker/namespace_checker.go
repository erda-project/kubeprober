package deployment_service_checker

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/pkg/kubeclient"
)

// Checker validates that deployment is functioning correctly
type NamespaceChecker struct {
	client  *kubernetes.Clientset
	Name    string
	Status  kubeproberv1.CheckerStatus
	Timeout time.Duration
}

// New returns a new DNS Checker
func NewChecker() (*NamespaceChecker, error) {
	// get kubernetes client
	client, err := kubeclient.Client(cfg.KubeConfigFile)
	if err != nil {
		logrus.Fatalln("Unable to create kubernetes client", err)
		return nil, err
	}
	return &NamespaceChecker{
		client:  client,
		Name:    "namespace-check",
		Timeout: cfg.CheckTimeout,
	}, nil
}

func (dc *NamespaceChecker) GetName() string {
	return dc.Name
}

func (dc *NamespaceChecker) SetName(n string) {
	dc.Name = n
}

func (dc *NamespaceChecker) GetStatus() kubeproberv1.CheckerStatus {
	return dc.Status
}

func (dc *NamespaceChecker) SetStatus(s kubeproberv1.CheckerStatus) {
	dc.Status = s
}

func (dc *NamespaceChecker) GetTimeout() time.Duration {
	return dc.Timeout
}

func (dc *NamespaceChecker) SetTimeout(t time.Duration) {
	dc.Timeout = t
}

// doChecks does validations on the DNS call to the endpoint
func (dc *NamespaceChecker) DoCheck() (err error) {
	ctx := context.Background()

	// namespace create
	err = createDeploymentNamespace(ctx, dc.client)
	if err != nil {
		return err
	}

	// namespace clean
	defer func() {
		err = deleteNamespace(ctx, dc.client)
		if err != nil {
			logrus.Errorf("clean resource, delete namespace failed, namespace: %s, error: %v", cfg.CheckNamespace, err)
			return
		}
	}()

	return nil
}
