package deployment_service_checker

import (
	"context"
	"errors"
	"math"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
)

const (
	defaultMinReadySeconds = 5

	// Default container values.
	defaultImagePullPolicy    = "IfNotPresent"
	defaultCheckContainerName = "deployment-container"

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

	err = createDeploymentNamespace(ctx, client)
	if err != nil {
		return err
	}

	log.Infoln("Creating deployment ", deploymentConfig.Name)

	deployment, err := client.AppsV1().Deployments(cfg.CheckNamespace).Create(ctx, deploymentConfig, metav1.CreateOptions{})
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
			// DnsLabelSelector: defaultLabelKey + "=" + defaultLabelValueBase + strconv.Itoa(int(now.Unix())),
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

// createDeploymentConfig creates and configures a k8s deployment and returns the struct (ready to apply with client).
func createDeploymentConfig() (*v1.Deployment, error) {
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
	labels := map[string]string{
		defaultCheckerKey:             defaultCheckerValue,
		kubeproberv1.DefaultSourceKey: kubeproberv1.DefaultSourceValue,
	}

	// Make and define a pod template spec with a pod spec.
	podTemplateSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cfg.CheckNamespace,
			Name:      cfg.CheckDeploymentName,
			Labels:    labels,
		},
		Spec: podSpec,
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

	deployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.CheckDeploymentName,
			Namespace: cfg.CheckNamespace,
		},
		Spec: v1.DeploymentSpec{
			Strategy:        deployStrategy,
			MinReadySeconds: defaultMinReadySeconds,
			Replicas:        &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: podTemplateSpec,
		},
	}

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

	// Make and define a resource requirement struct.
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse(cfg.CpuRequest),
			"memory": resource.MustParse(cfg.MemoryRequest),
		},
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse(cfg.CpuLimit),
			"memory": resource.MustParse(cfg.MemoryLimit),
		},
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
