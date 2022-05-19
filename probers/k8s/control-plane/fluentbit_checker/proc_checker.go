package fluentbit_checker

import (
	"context"
	"time"

	"github.com/pkg/errors"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/pkg/kubeclient"
	"github.com/erda-project/kubeprober/probers/k8s/control-plane/config"
	"github.com/sirupsen/logrus"
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

	if !checkControlVersion(pods) {
		logrus.Infof("controller are not the same version skip checker, %v", pods)
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

	var errors []error

	for _, item := range pods.Items {

		first, ok := firstMetric[item.Name]
		if !ok {
			logrus.Infof("pod%v, not found metric %v", item.Name, firstMetric)
			continue
		}
		last, ok := lastMetric[item.Name]
		if !ok {
			logrus.Infof("pod%v, not found metric %v", item.Name, lastMetric)
			continue
		}

		firstInput, firstOutput := first.totalProc()
		lastInput, lastOutput := last.totalProc()

		if firstInput <= 0 && lastInput <= 0 {
			// no input, no check
			continue
		}

		if firstOutput == lastOutput {
			logrus.Infof("fluent-bit is hang! %v;%v", firstOutput, lastOutput)
			// output = output,no process during sampling
			err := selfHealing(ctx, f.client, item)
			if err != nil {
				logrus.Errorf("failed to self healing ,error %v", err)
				errors = append(errors, err)
				continue
			}
		}
	}
	if len(errors) > 0 {
		return condenseErrors(errors)
	}
	return nil
}

func condenseErrors(errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	}
	err := errs[0]
	for _, e := range errs[1:] {
		err = errors.Wrap(err, e.Error())
	}
	return err
}

var nowTime = time.Now()

func checkPodRestart(pod apiv1.Pod) bool {
	restartTime, ok := pod.Annotations["kubectl.kubernetes.io/restartedAt"]
	if !ok {
		// maybe k8s version is slow, no check restart
		logrus.Warnf("not found oid restartat. skip check. %v", pod)
		return true
	}
	//restartAt, err := time.Parse("2006-01-02T15:04:05+07:00", restartTime)
	restartAt, err := time.Parse(time.RFC3339, restartTime)
	if err != nil {
		logrus.Warnf("failed to check pod restart parse restartAt,value: %v,error:%v", restartTime, err)
		return true
	}
	if nowTime.Sub(restartAt) < cfg.FluentBitRestartProtectionTime {
		logrus.Errorf("pod is in protection phase after restart, %v", pod)
		return false
	}
	return true
}

func checkControlVersion(pods *apiv1.PodList) bool {
	// controller-revision-hash
	// https://github.com/kubernetes/kubernetes/issues/47554
	// controller version no group, maybe in the processing of rollout
	// Skip the entire restart case of the daemonset
	if len(pods.Items) <= 0 {
		return true
	}
	version, ok := pods.Items[0].Labels["controller-revision-hash"]
	if !ok {
		//maybe k8s version is too slow, don`t not doing check
		return true
	}
	for _, item := range pods.Items {
		podVersion, ok := item.Labels["controller-revision-hash"]
		if !ok {
			continue
		}
		if podVersion != version {
			return false
		}
	}
	return true
}

func selfHealing(ctx context.Context, client *kubernetes.Clientset, pod apiv1.Pod) error {
	if !cfg.FluentBitSelfHealingEnable {
		return errors.New("fluent-bit is hang")
	}

	if !checkPodRestart(pod) {
		logrus.Infof("fluent-bit is hang, but too much restart, %v", pod)
		return nil
	}
	err := client.CoreV1().Pods(cfg.FluentBitNamespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
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
			logrus.Warnf("error to get metric by pod id,but is hidden error:%v", err)
			continue
		}
		metrics[pod.Name] = metric
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
