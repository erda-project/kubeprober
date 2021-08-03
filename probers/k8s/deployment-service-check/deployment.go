package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
)

// Checker validates that deployment is functioning correctly
type DeployServiceChecker struct {
	client  *kubernetes.Clientset
	Name    string
	Status  kubeproberv1.CheckerStatus
	Timeout time.Duration
}

const (
	// Default deployment values.
	defaultLabelKey        = "deployment-timestamp"
	defaultLabelValueBase  = "unix-"
	defaultMinReadySeconds = 5

	// Default deployment strategy values.
	defaultMaxSurge       = 2
	defaultMaxUnavailable = 2

	// Default container values.
	defaultImagePullPolicy    = "IfNotPresent"
	defaultCheckContainerName = "deployment-container"

	// Default container resource requests values.
	defaultMillicoreRequest = 15               // Calculated in decimal SI units (15 = 15m cpu).
	defaultMillicoreLimit   = 75               // Calculated in decimal SI units (75 = 75m cpu).
	defaultMemoryRequest    = 20 * 1024 * 1024 // Calculated in binary SI units (20 * 1024^2 = 20Mi memory).
	defaultMemoryLimit      = 75 * 1024 * 1024 // Calculated in binary SI units (75 * 1024^2 = 75Mi memory).

	// Default container probe values.
	defaultProbeFailureThreshold    = 5  // Number of consecutive failures for the probe to be considered failed (k8s default = 3).
	defaultProbeSuccessThreshold    = 1  // Number of consecutive successes for the probe to be considered successful after having failed (k8s default = 1).
	defaultProbeInitialDelaySeconds = 2  // Number of seconds after container has started before probes are initiated.
	defaultProbeTimeoutSeconds      = 2  // Number of seconds after which the probe times out (k8s default = 1).
	defaultProbePeriodSeconds       = 15 // How often to perform the probe (k8s default = 10).
)

// createDeployment creates a deployment in the cluster with a given deployment specification.
func createDeployment(ctx context.Context, client *kubernetes.Clientset) error {
	deploymentConfig, err := createDeploymentConfig()
	if err != nil {
		log.Errorf("create deployment config failed, err: %v", err)
		return err
	}

	log.Infoln("Creating deployment in cluster with name:", deploymentConfig.Name)

	deployment, err := client.AppsV1().Deployments(cfg.CheckDeploymentName).Create(ctx, deploymentConfig, metav1.CreateOptions{})
	if err != nil {
		log.Errorf("Failed to create deployment, err: %v", err)
		return err
	}
	if deployment == nil {
		err = errors.New("got a nil deployment result: ")
		log.Errorln("Failed to create a deployment in the cluster: %w", err)
		return err
	}

	for {
		log.Infoln("Watching for deployment to exist.")

		// Watch that it is up.
		watch, err := client.AppsV1().Deployments(cfg.CheckNamespace).Watch(ctx, metav1.ListOptions{
			Watch:         true,
			FieldSelector: "metadata.name=" + deployment.Name,
			// LabelSelector: defaultLabelKey + "=" + defaultLabelValueBase + strconv.Itoa(int(now.Unix())),
		})
		if err != nil {
			return err
		}
		// If the watch is nil, skip to the next loop and make a new watch object.
		if watch == nil {
			time.Sleep(5 * time.Second)
			continue
		}

		// There can be 2 events here: Available = True status update from deployment or Context timeout.
		for event := range watch.ResultChan() { // Watch for deployment events.

			d, ok := event.Object.(*v1.Deployment)
			if !ok { // Skip the event if it cannot be casted as a v1.Deployment.
				log.Infoln("Got a watch event for a non-deployment object -- ignoring.")
				continue
			}

			log.Debugln("Received an event watching for deployment changes:", d.Name, "got event", event.Type)

			// Look at the status conditions for the deployment object;
			// we want it to be reporting Available = True.
			if deploymentAvailable(d) {
				return nil
			}
		}

		// Stop the watch on each loop because we will create a new one.
		watch.Stop()
	}
}

// deleteDeploymentAndWait deletes the created test deployment
func deleteDeploymentAndWait(ctx context.Context, client *kubernetes.Clientset) error {

	deleteChan := make(chan error)

	go func() {
		defer close(deleteChan)

		log.Debugln("Checking if deployment has been deleted.")
		for {

			// Check if we have timed out.
			select {
			case <-ctx.Done():
				deleteChan <- fmt.Errorf("timed out while waiting for deployment to delete")
			default:
				log.Debugln("Delete deployment and wait has not yet timed out.")
			}

			// Wait between checks.
			log.Debugln("Waiting 5 seconds before trying again.")
			time.Sleep(time.Second * 5)

			// Watch that it is gone by listing repeatedly.
			deploymentList, err := client.AppsV1().Deployments(cfg.CheckNamespace).List(ctx, metav1.ListOptions{
				FieldSelector: "metadata.name=" + cfg.CheckDeploymentName,
				// LabelSelector: defaultLabelKey + "=" + defaultLabelValueBase + strconv.Itoa(int(now.Unix())),
			})
			if err != nil {
				log.Errorln("Error listing deployments:", err.Error())
				continue
			}

			// Check for the deployment in the list.
			var deploymentExists bool
			for _, deploy := range deploymentList.Items {
				// If the deployment exists, try to delete it.
				if deploy.GetName() == cfg.CheckDeploymentName {
					deploymentExists = true
					err = deleteDeployment(ctx, client)
					if err != nil {
						log.Errorln("Error when running a delete on deployment", cfg.CheckDeploymentName+":", err.Error())
					}
					break
				}
			}

			// If the deployment was not in the list, then we assume it has been deleted.
			if !deploymentExists {
				deleteChan <- nil
				break
			}
		}

	}()

	// Send a delete on the deployment.
	err := deleteDeployment(ctx, client)
	if err != nil {
		log.Infoln("Could not delete deployment:", cfg.CheckDeploymentName)
	}

	return <-deleteChan
}

// deleteDeployment issues a foreground delete for the check test deployment name.
func deleteDeployment(ctx context.Context, client *kubernetes.Clientset) error {
	log.Infoln("Attempting to delete deployment in", cfg.CheckNamespace, "namespace.")
	// Make a delete options object to delete the deployment.
	deletePolicy := metav1.DeletePropagationForeground
	graceSeconds := int64(1)
	deleteOpts := metav1.DeleteOptions{
		GracePeriodSeconds: &graceSeconds,
		PropagationPolicy:  &deletePolicy,
	}

	// Delete the deployment and return the result.
	return client.AppsV1().Deployments(cfg.CheckNamespace).Delete(ctx, cfg.CheckDeploymentName, deleteOpts)
}

// updateDeployment performs an update on a deployment with a given deployment configuration.  The DeploymentResult
// channel is notified when the rolling update is complete.
func updateDeployment(ctx context.Context, client *kubernetes.Clientset, deploymentConfig *v1.Deployment) error {

	updateChan := make(chan error)

	go func() {
		log.Infoln("Performing rolling-update on deployment", deploymentConfig.Name, "to ["+deploymentConfig.Spec.Template.Spec.Containers[0].Image+"]")

		defer close(updateChan)

		// Get the names of the current pods and ignore them when checking for a completed rolling-update.
		// log.Infoln("Creating a blacklist with the current pods that exist.")
		// oldPodNames := getPodNames()
		// newPodStatuses := make(map[string]bool)

		deployment, err := client.AppsV1().Deployments(cfg.CheckNamespace).Update(ctx, deploymentConfig, metav1.UpdateOptions{})
		if err != nil {
			log.Infoln("Failed to update deployment in the cluster:", err)
			updateChan <- err
			return
		}

		// Watch that it is up.
		watch, err := client.AppsV1().Deployments(cfg.CheckNamespace).Watch(ctx, metav1.ListOptions{
			Watch:         true,
			FieldSelector: "metadata.name=" + deployment.Name,
			// LabelSelector: defaultLabelKey + "=" + defaultLabelValueBase + strconv.Itoa(int(now.Unix())),
		})
		if err != nil {
			updateChan <- err
			return
		}

		// Stop the watch on each loop because we will create a new one.
		defer watch.Stop()

		log.Debugln("Watching for deployment rolling-update to complete.")
		for {
			select {
			case event := <-watch.ResultChan():
				// Watch for deployment events.
				d, ok := event.Object.(*v1.Deployment)
				if !ok { // Skip the event if it cannot be casted as a v1.Deployment.
					log.Infoln("Got a watch event for a non-deployment object -- ignoring.")
					continue
				}

				log.Debugln("Received an event watching for deployment changes:", d.Name, "got event", event.Type)

				if rolledPodsAreReady(d) {
					log.Debugln("Rolling-update is assumed to be completed, sending result to channel.")
					updateChan <- nil
					return
				}

			case <-ctx.Done():
				// If the context has expired, exit.
				log.Errorln("Context expired while waiting for deployment to create.")
				// TODO:
				updateChan <- nil
				return
			}
		}
	}()

	return <-updateChan
}

// waitForDeploymentToDelete waits for the service to be deleted.
func waitForDeploymentToDelete(ctx context.Context, client *kubernetes.Clientset) chan bool {

	// Make and return a channel while we check that the service is gone in the background.
	deleteChan := make(chan bool, 1)

	go func() {
		defer close(deleteChan)
		for {
			_, err := client.AppsV1().Deployments(cfg.CheckNamespace).Get(ctx, cfg.CheckDeploymentName, metav1.GetOptions{})
			if err != nil {
				log.Debugln("error from Deployments().Get():", err.Error())
				if k8sErrors.IsNotFound(err) || strings.Contains(err.Error(), "not found") {
					log.Debugln("Deployment deleted.")
					deleteChan <- true
					return
				}
			}
			time.Sleep(time.Millisecond * 250)
		}
	}()

	return deleteChan
}

// createDeploymentConfig creates and configures a k8s deployment and returns the struct (ready to apply with client).
func createDeploymentConfig() (*v1.Deployment, error) {

	// Make a k8s deployment.
	deployment := &v1.Deployment{}

	// Use a different image if useRollImage is true, to
	checkImage := cfg.CheckImage

	log.Infoln("Creating deployment resource with", cfg.CheckDeploymentReplicas, "replica(s) in", cfg.CheckNamespace, "namespace using image ["+checkImage+"] with environment variables:", cfg.CheckDeploymentAdditionalEnvs)

	// Make a slice for containers for the pods in the deployment.
	containers := make([]corev1.Container, 0)

	if len(checkImage) == 0 {
		err := errors.New("check image url for container is empty: " + checkImage)
		log.Errorf(err.Error())
		return nil, err
	}

	// Make the container for the slice.
	var container corev1.Container
	container = createContainerConfig()
	containers = append(containers, container)

	// Check for given node selector values.
	// Set the map to the default of nil (<none>) if there are no selectors given.
	if len(cfg.CheckDeploymentNodeSelectors) == 0 {
		cfg.CheckDeploymentNodeSelectors = nil
	}

	graceSeconds := int64(1)

	// Make and define a pod spec with containers.
	podSpec := corev1.PodSpec{
		Containers:                    containers,
		NodeSelector:                  cfg.CheckDeploymentNodeSelectors,
		RestartPolicy:                 corev1.RestartPolicyAlways,
		TerminationGracePeriodSeconds: &graceSeconds,
		ServiceAccountName:            cfg.CheckServiceAccount,
		Tolerations:                   cfg.CheckDeploymentToleration,
	}

	// Make labels for pod and deployment.
	labels := make(map[string]string, 0)
	// TODO: labels
	labels["source"] = "kuberhealthy"

	// Make and define a pod template spec with a pod spec.
	podTemplateSpec := corev1.PodTemplateSpec{
		Spec: podSpec,
	}
	podTemplateSpec.ObjectMeta.Labels = labels
	podTemplateSpec.ObjectMeta.Name = cfg.CheckDeploymentName
	podTemplateSpec.ObjectMeta.Namespace = cfg.CheckNamespace

	// Make a selector object for labels.
	labelSelector := metav1.LabelSelector{
		MatchLabels: labels,
	}

	// Calculate max surge and unavailable [#replicas / 2].
	maxSurge := math.Ceil(float64(cfg.CheckDeploymentReplicas) / float64(2))
	maxUnavailable := math.Ceil(float64(cfg.CheckDeploymentReplicas) / float64(2))

	// Make a rolling update strategy and define the deployment strategy with it.
	rollingUpdateSpec := v1.RollingUpdateDeployment{
		MaxUnavailable: &intstr.IntOrString{
			IntVal: int32(maxUnavailable),
			StrVal: strconv.Itoa(int(maxUnavailable)),
		},
		MaxSurge: &intstr.IntOrString{
			IntVal: int32(maxSurge),
			StrVal: strconv.Itoa(int(maxSurge)),
		},
	}
	deployStrategy := v1.DeploymentStrategy{
		Type:          v1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &rollingUpdateSpec,
	}

	// Make a deployment spec.
	replicas := int32(cfg.CheckDeploymentReplicas)
	deploySpec := v1.DeploymentSpec{
		Strategy:        deployStrategy,
		MinReadySeconds: defaultMinReadySeconds,
		Replicas:        &replicas,
		Selector:        &labelSelector,
		Template:        podTemplateSpec,
	}

	// Define the k8s deployment.
	deployment.ObjectMeta.Name = cfg.CheckDeploymentName
	deployment.ObjectMeta.Namespace = cfg.CheckNamespace

	// Add the deployment spec to the deployment.
	deployment.Spec = deploySpec

	return deployment, nil
}

// createContainerConfig creates a container resource spec and returns it.
func createContainerConfig() corev1.Container {

	log.Infoln("Creating container using image ["+cfg.CheckImage+"] with environment variables:", cfg.CheckAdditionalEnvs)

	// Set up a basic container port [default is 80 for HTTP].
	basicPort := corev1.ContainerPort{
		ContainerPort: cfg.CheckContainerPort,
	}
	containerPorts := []corev1.ContainerPort{basicPort}

	// Make maps for resources.
	// Make and define a map for requests.
	// TODO: check
	requests := make(map[corev1.ResourceName]resource.Quantity, 0)
	requests[corev1.ResourceCPU] = *resource.NewMilliQuantity(int64(cfg.CpuRequest), resource.DecimalSI)
	requests[corev1.ResourceMemory] = *resource.NewQuantity(int64(cfg.MemoryRequest), resource.BinarySI)

	// Make and define a map for limits.
	limits := make(map[corev1.ResourceName]resource.Quantity, 0)
	limits[corev1.ResourceCPU] = *resource.NewMilliQuantity(int64(cfg.CpuLimit), resource.DecimalSI)
	limits[corev1.ResourceMemory] = *resource.NewQuantity(int64(cfg.MemoryLimit), resource.BinarySI)

	// Make and define a resource requirement struct.
	resources := corev1.ResourceRequirements{
		Requests: requests,
		Limits:   limits,
	}

	// Make a slice for environment variables.
	// Parse passed in environment variables and define the slice.
	envs := make([]corev1.EnvVar, 0)
	for k, v := range cfg.CheckDeploymentAdditionalEnvs {
		ev := corev1.EnvVar{
			Name:  k,
			Value: v,
		}
		envs = append(envs, ev)
	}

	// Make a TCP socket for the probe handler.
	tcpSocket := corev1.TCPSocketAction{
		Port: intstr.IntOrString{
			IntVal: cfg.CheckContainerPort,
			StrVal: strconv.Itoa(int(cfg.CheckContainerPort)),
		},
	}

	// Make a handler for the probes.
	handler := corev1.Handler{
		TCPSocket: &tcpSocket,
	}

	// Make liveness and readiness probes.
	// Make the liveness probe here.
	liveProbe := corev1.Probe{
		Handler:             handler,
		InitialDelaySeconds: defaultProbeInitialDelaySeconds,
		TimeoutSeconds:      defaultProbeTimeoutSeconds,
		PeriodSeconds:       defaultProbePeriodSeconds,
		SuccessThreshold:    defaultProbeSuccessThreshold,
		FailureThreshold:    defaultProbeFailureThreshold,
	}

	// Make the readiness probe here.
	readyProbe := corev1.Probe{
		Handler:             handler,
		InitialDelaySeconds: defaultProbeInitialDelaySeconds,
		TimeoutSeconds:      defaultProbeTimeoutSeconds,
		PeriodSeconds:       defaultProbePeriodSeconds,
		SuccessThreshold:    defaultProbeSuccessThreshold,
		FailureThreshold:    defaultProbeFailureThreshold,
	}

	// Create the container.
	c := corev1.Container{
		Name:            defaultCheckContainerName,
		Image:           cfg.CheckImage,
		ImagePullPolicy: defaultImagePullPolicy,
		Ports:           containerPorts,
		Resources:       resources,
		Env:             envs,
		LivenessProbe:   &liveProbe,
		ReadinessProbe:  &readyProbe,
	}

	return c
}

// rollingUpdateComplete checks the deployment's container images and their statuses and returns
// a boolean based on whether or not the rolling-update is complete.
func rollingUpdateComplete(ctx context.Context, client *kubernetes.Clientset, statuses map[string]bool, oldPodNames []string) bool {

	// Should be looking at pod and pod names NOT containers.
	podList, err := client.CoreV1().Pods(cfg.CheckNamespace).List(ctx, metav1.ListOptions{
		// FieldSelector: "metadata.name=" + checkDeploymentName,
		// TODO: pod label
		LabelSelector: defaultLabelKey + "=" + defaultLabelValueBase,
	})
	if err != nil {
		log.Errorln("failed to list pods:", err)
		return false
	}

	// Look at each pod and see if the deployment update is complete.
	for _, pod := range podList.Items {
		if containsString(pod.Name, oldPodNames) {
			log.Debugln("Skipping", pod.Name, "because it was found in the blacklist.")
			continue
		}

		// If the container in the pod has the correct image, add it to the status map.
		for _, container := range pod.Spec.Containers {
			if container.Image == cfg.CheckImageRoll {
				if _, ok := statuses[pod.Name]; !ok {
					statuses[pod.Name] = false
				}
			}
		}

		// Check the pod conditions to see if it has finished the rolling-update.
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				if !statuses[pod.Name] {
					log.Debugln("Setting status for", pod.Name, "to true.")
					statuses[pod.Name] = true
				}
			}
		}
	}

	var count int
	for _, status := range statuses {
		if status {
			count++
		}
	}
	log.Infoln(count, "/", cfg.CheckDeploymentReplicas, "pods have been rolled.")

	// Only return true if ALL pods are up.
	return count == cfg.CheckDeploymentReplicas
}

// rolledPodsAreReady checks if a deployments pods have been updated and are available.
// Returns true if all replicas are up, ready, and the deployment generation is greater than 1.
func rolledPodsAreReady(d *v1.Deployment) bool {
	return d.Status.Replicas == int32(cfg.CheckDeploymentReplicas) &&
		d.Status.AvailableReplicas == int32(cfg.CheckDeploymentReplicas) &&
		d.Status.ReadyReplicas == int32(cfg.CheckDeploymentReplicas) &&
		d.Status.ObservedGeneration > 1
}

// deploymentAvailable checks the status conditions of the deployment and returns a boolean.
// This will return a true if condition 'Available' = status 'True'.
func deploymentAvailable(deployment *v1.Deployment) bool {
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == v1.DeploymentAvailable && condition.Status == corev1.ConditionTrue {
			log.Infoln("Deployment is reporting", condition.Type, "with", condition.Status+".")
			return true
		}
	}
	return false
}

// containsString returns a boolean value based on whether or not a slice of strings contains
// a string.
func containsString(s string, list []string) bool {
	for _, str := range list {
		if s == str {
			return true
		}
	}
	return false
}
