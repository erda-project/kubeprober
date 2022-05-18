package fluentbit_checker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/pkg/kubeclient"
	"github.com/erda-project/kubeprober/probers/k8s/control-plane/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type FluentbitProcessChecker struct {
	Name    string
	Timeout time.Duration
	Status  kubeproberv1.CheckerStatus
	client  *kubernetes.Clientset
}

func NewChecker() (*FluentbitProcessChecker, error) {
	client, err := kubeclient.Client(cfg.KubeConfigFile)
	if err != nil {
		logrus.Fatalln("Unable to create kubernetes client", err)
		return nil, err
	}

	c := &FluentbitProcessChecker{
		Name:    "fluentbit-process-checker",
		Timeout: config.Cfg.CheckTimeout,
		client:  client,
	}
	return c, nil
}

func (f FluentbitProcessChecker) GetName() string {
	return f.Name
}

func (f FluentbitProcessChecker) SetName(s string) {
	f.Name = s
}

func (f FluentbitProcessChecker) GetStatus() kubeproberv1.CheckerStatus {
	return f.Status
}

func (f FluentbitProcessChecker) SetStatus(status kubeproberv1.CheckerStatus) {
	f.Status = status
}

func (f FluentbitProcessChecker) GetTimeout() time.Duration {
	return f.Timeout
}

func (f FluentbitProcessChecker) SetTimeout(duration time.Duration) {
	f.Timeout = duration
}

func (f FluentbitProcessChecker) DoCheck() error {
	ctx := context.Background()
	pods, err := getFluentBitPods(ctx, f.client)
	if err != nil {
		logrus.Errorf("failed to get pods error:%v", err)
		return err
	}
	if len(pods.Items) <= 0 {
		logrus.Info("no pod")
		return nil
	}
	firstMetric, err := getMetricByPods(pods)
	if err != nil {
		logrus.Errorf("failed to get first metric, error:%v", err)
		return err
	}

	logrus.Infof("start sampling, time is %v", cfg.FluentBitSampling)
	time.Sleep(cfg.FluentBitSampling)

	lastMetric, err := getMetricByPods(pods)
	if err != nil {
		logrus.Errorf("failed to get last metric, error: %v", err)
		return err
	}

	firstInput, firstOutput := totalProc(firstMetric)
	lastInput, lastOutput := totalProc(lastMetric)

	if firstInput <= 0 && lastInput <= 0 {
		// no input, no check
		return nil
	}

	if firstOutput == lastOutput {
		logrus.Infof("fluent-bit is hang! %v;%v", firstOutput, lastOutput)
		// output = output,no process during sampling
		err := SelfHealing(ctx, f.client)
		if err != nil {
			logrus.Errorf("failed to self healing ,error %v", err)
			return err
		}
	}
	return nil
}

func SelfHealing(ctx context.Context, client *kubernetes.Clientset) error {
	if !cfg.FluentBitSelfHealingEnable {
		return errors.New("fluent-bit is hang")
	}
	patchBody := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"kubectl.kubernetes.io/restartedAt": time.Now().Format("2006-01-02T15:04:05+07:00"),
					},
				},
			},
		},
	}

	data, err := json.Marshal(patchBody)
	if err != nil {
		return fmt.Errorf("failed to json serialization , error %v", err)
	}
	_, err = client.AppsV1().DaemonSets(cfg.FluentBitNamespace).Patch(ctx, cfg.FluentBitDaemonSetName, types.StrategicMergePatchType, data, metav1.PatchOptions{})
	return err
}
func getMetricByPods(pods *apiv1.PodList) (map[string]*FluentBitMetric, error) {
	metrics := make(map[string]*FluentBitMetric)

	for _, pod := range pods.Items {
		if pod.Status.Phase != apiv1.PodRunning {
			continue
		}
		podIp := pod.Status.PodIP
		if len(podIp) <= 0 {
			continue
		}
		metric, err := getMetricByIp(podIp)
		if err != nil {
			return nil, err
		}
		metrics[podIp] = metric
	}
	return metrics, nil
}

func getFluentBitPods(ctx context.Context, client *kubernetes.Clientset) (*apiv1.PodList, error) {
	pods := &apiv1.PodList{}
	listOpt := metav1.ListOptions{
		Limit:         100,
		LabelSelector: cfg.FluentBitLabelSelector,
	}

	for {
		page, err := client.CoreV1().Pods(cfg.FluentBitNamespace).List(ctx, listOpt)
		if err != nil {
			return nil, err
		}
		pods.Items = append(pods.Items, page.Items...)
		if page.Continue != "" {
			listOpt.Continue = page.Continue
		} else {
			break
		}
	}
	return pods, nil
}
