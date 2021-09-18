package deployment_service_checker

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func createDeploymentNamespace(ctx context.Context, client *kubernetes.Clientset) error {
	// check namespace, delete it if exist
	_, err := client.CoreV1().Namespaces().Get(ctx, CheckNewNamespace, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			logrus.Infof("namespace [%s] not found, create it", CheckNewNamespace)
		} else {
			logrus.Errorf("get namespace failed, namespace: %s, error: %v", CheckNewNamespace, err)
			return err
		}
	} else {
		err := deleteNamespace(ctx, client)
		if err != nil {
			log.Errorf("delete previous namespaces failed, namespace: %s, error: %v", CheckNewNamespace, err)
			return err
		}
	}

	// create namespace
	err = createNamespace(ctx, client)
	if err != nil {
		log.Errorf("create namespace failed, namespace: %s, error: %v", CheckNewNamespace, err)
		return err
	}
	log.Infof("create namespace successfully, namespace: %s", CheckNewNamespace)
	return nil
}

func createNamespace(ctx context.Context, client *kubernetes.Clientset) error {
	// create namespace
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: CheckNewNamespace,
		},
	}
	_, err := client.CoreV1().Namespaces().Create(ctx, &namespace, metav1.CreateOptions{})
	if err != nil {
		logrus.Errorf("create namespace failed, namespace: %s, error: %v", CheckNewNamespace, err)
		return err
	}

	for {
		logrus.Infoln("Watching for namespace ready.")

		// Watch that it is up.
		watch, err := client.CoreV1().Namespaces().Watch(ctx, metav1.ListOptions{
			Watch:         true,
			FieldSelector: "metadata.name=" + CheckNewNamespace,
		})
		if err != nil {
			return err
		}
		// If the watch is nil, skip to the next loop and make a new watch object.
		if watch == nil {
			time.Sleep(3 * time.Second)
			continue
		}

		for event := range watch.ResultChan() { // Watch for deployment events.

			n, ok := event.Object.(*corev1.Namespace)
			if !ok {
				logrus.Infoln("Got a watch event for a non-namespace object -- ignoring.")
				continue
			}

			logrus.Debugln("Received an event watching for namespace changes:", n.Name, "got event", event.Type)

			// Look at the status conditions for the deployment object;
			// we want it to be reporting Available = True.
			if namespaceAvailable(n) {
				return nil
			}
		}

		// Stop the watch on each loop because we will create a new one.
		watch.Stop()
	}
}

func deleteNamespace(ctx context.Context, client *kubernetes.Clientset) error {
	period := int64(0)

	_, err := client.CoreV1().Namespaces().Get(ctx, CheckNewNamespace, metav1.GetOptions{})
	if err != nil && k8sErrors.IsNotFound(err) {
		logrus.Infof("namespace deleted")
		return nil
	}

	err = client.CoreV1().Namespaces().Delete(ctx, CheckNewNamespace, metav1.DeleteOptions{GracePeriodSeconds: &period})
	if err != nil {
		logrus.Errorf("delete namespace failed, namespace: %s, error: %v", CheckNewNamespace, err)
		return err
	}
	for {
		logrus.Infoln("Watching for namespace deleted.")
		time.Sleep(3 * time.Second)
		_, err := client.CoreV1().Namespaces().Get(ctx, CheckNewNamespace, metav1.GetOptions{})
		if err != nil && k8sErrors.IsNotFound(err) {
			logrus.Infof("namespace deleted")
			return nil
		}
	}
}

func namespaceAvailable(n *corev1.Namespace) bool {
	return n.Status.Phase == corev1.NamespaceActive
}
