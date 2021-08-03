package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"k8s.io/client-go/kubernetes"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// createService creates a deployment in the cluster with a given deployment specification.
func createService(ctx context.Context, client *kubernetes.Clientset, labels map[string]string) error {

	serviceConfig := createServiceConfig(labels)

	createChan := make(chan error)

	go func() {
		log.Infoln("Creating service in cluster with name:", serviceConfig.Name)

		defer close(createChan)

		service, err := client.CoreV1().Services(cfg.CheckNamespace).Create(ctx, serviceConfig, metav1.CreateOptions{})
		if err != nil {
			log.Infoln("Failed to create a service in the cluster:", err)
			createChan <- err
			return
		}
		if service == nil {
			err = errors.New("got a nil service result: ")
			log.Errorln("Failed to create a service in the cluster: %w", err)
			createChan <- err
		}

		for {
			log.Infoln("Watching for service to exist.")

			// Watch that it is up.
			watch, err := client.CoreV1().Services(cfg.CheckNamespace).Watch(ctx, metav1.ListOptions{
				Watch:         true,
				FieldSelector: "metadata.name=" + service.Name,
				// LabelSelector: defaultLabelKey + "=" + defaultLabelValueBase + strconv.Itoa(int(now.Unix())),
			})
			if err != nil {
				createChan <- err
				return
			}
			// If the watch is nil, skip to the next loop and make a new watch object.
			if watch == nil {
				continue
			}

			// There can be 2 events here: Service ingress has at least 1 hostname endpoint or Context timeout.
			select {
			case event := <-watch.ResultChan():
				log.Debugln("Received an event watching for service changes:", event.Type)

				s, ok := event.Object.(*corev1.Service)
				if !ok { // Skip the event if it cannot be casted as a corev1.Service
					log.Debugln("Got a watch event for a non-service object -- ignoring.")
					continue
				}

				// Look at the length of the ClusterIP.
				if serviceAvailable(s) {
					createChan <- nil
					return
				}
			case <-serviceHasClusterIP(ctx, client):
				log.Debugln("A cluster IP belonging to the created service has been found:")
				createChan <- nil
				return
			case <-ctx.Done():
				log.Errorln("context expired while waiting for service to create.")
				createChan <- nil
				return
			}

			watch.Stop()
		}
	}()

	return <-createChan
}

// deleteServiceAndWait deletes the created test service.
func deleteServiceAndWait(ctx context.Context, client *kubernetes.Clientset) error {

	deleteChan := make(chan error)

	go func() {
		defer close(deleteChan)

		log.Debugln("Checking if service has been deleted.")
		for {

			// Check if we have timed out.
			select {
			case <-ctx.Done():
				deleteChan <- fmt.Errorf("timed out while waiting for service to delete")
				return
			default:
				log.Debugln("Delete service and wait has not yet timed out.")
			}

			// Wait between checks.
			log.Debugln("Waiting 5 seconds before trying again.")
			time.Sleep(time.Second * 5)

			// Watch that it is gone by listing repeatedly.
			serviceList, err := client.CoreV1().Services(cfg.CheckNamespace).List(ctx, metav1.ListOptions{
				FieldSelector: "metadata.name=" + cfg.CheckServiceName,
				// LabelSelector: defaultLabelKey + "=" + defaultLabelValueBase + strconv.Itoa(int(now.Unix())),
			})
			if err != nil {
				log.Errorln("Error creating service listing client:", err.Error())
				continue
			}

			// Check for the service in the list.
			var serviceExists bool
			for _, svc := range serviceList.Items {
				// If the service exists, try to delete it.
				if svc.GetName() == cfg.CheckServiceName {
					serviceExists = true
					err = deleteService(ctx, client)
					if err != nil {
						log.Errorln("Error when running a delete on service", cfg.CheckServiceName+":", err.Error())
					}
					break
				}
			}

			// If the service was not in the list, then we assume it has been deleted.
			if !serviceExists {
				deleteChan <- nil
				break
			}
		}

	}()

	// Send a delete on the service.
	err := deleteService(ctx, client)
	if err != nil {
		log.Infoln("Could not delete service:", cfg.CheckServiceName)
	}

	return <-deleteChan
}

// deleteService issues a foreground delete for the check test service name.
func deleteService(ctx context.Context, client *kubernetes.Clientset) error {
	log.Infoln("Attempting to delete service", cfg.CheckServiceName, "in", cfg.CheckNamespace, "namespace.")
	// Make a delete options object to delete the service.
	deletePolicy := metav1.DeletePropagationForeground
	graceSeconds := int64(1)
	deleteOpts := metav1.DeleteOptions{
		GracePeriodSeconds: &graceSeconds,
		PropagationPolicy:  &deletePolicy,
	}

	// Delete the service and return the result.
	return client.CoreV1().Services(cfg.CheckNamespace).Delete(ctx, cfg.CheckServiceName, deleteOpts)
}

// getServiceClusterIP retrieves the cluster IP address utilized for the service
func getServiceClusterIP(ctx context.Context, client *kubernetes.Clientset) string {

	svc, err := client.CoreV1().Services(cfg.CheckNamespace).Get(ctx, cfg.CheckServiceName, metav1.GetOptions{})
	if err != nil {
		log.Errorln("Error occurred attempting to list service while retrieving service cluster IP:", err)
		return ""
	}
	if svc == nil {
		log.Errorln("Failed to get service, received a nil object:", svc)
		return ""
	}

	log.Debugln("Retrieving a cluster IP belonging to:", svc.Name)
	if len(svc.Spec.ClusterIP) != 0 {
		log.Infoln("Found service cluster IP address:", svc.Spec.ClusterIP)
		return svc.Spec.ClusterIP
	}
	return ""
}

// serviceAvailable checks the amount of ingress endpoints associated to the service.
// This will return a true if there is at least 1 hostname endpoint.
func serviceAvailable(service *corev1.Service) bool {
	if service == nil {
		return false
	}
	if len(service.Spec.ClusterIP) != 0 {
		log.Infoln("Cluster IP found:", service.Spec.ClusterIP)
		return true
	}
	return false
}

// serviceHasClusterIP checks the service object to see if a cluster IP has been
// allocated to it yet and returns when a cluster IP exists.
func serviceHasClusterIP(ctx context.Context, client *kubernetes.Clientset) chan *corev1.Service {

	resultChan := make(chan *corev1.Service)

	go func() {
		defer close(resultChan)

		for {
			svc, err := client.CoreV1().Services(cfg.CheckNamespace).Get(ctx, cfg.CheckServiceName, metav1.GetOptions{})
			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			if len(svc.Spec.ClusterIP) != 0 {
				resultChan <- svc
				return
			}
		}
	}()

	return resultChan
}

// createServiceConfig creates and configures a k8s service and returns the struct (ready to apply with client).
func createServiceConfig(labels map[string]string) *corev1.Service {

	// Make a k8s service.
	service := &corev1.Service{}

	log.Infoln("Creating service resource for", cfg.CheckNamespace, "namespace.")

	// Make and define a port for the service.
	ports := make([]corev1.ServicePort, 0)
	basicPort := corev1.ServicePort{
		Port: cfg.CheckLoadBalancerPort, // Port to hit the load balancer on.
		TargetPort: intstr.IntOrString{ // Port to hit the container on.
			IntVal: cfg.CheckContainerPort,
			StrVal: strconv.Itoa(int(cfg.CheckContainerPort)),
		},
		Protocol: corev1.ProtocolTCP,
	}
	ports = append(ports, basicPort)

	// Make a service spec.
	serviceSpec := corev1.ServiceSpec{
		Type:     corev1.ServiceTypeClusterIP,
		Ports:    ports,
		Selector: labels,
	}

	// Define the service.
	service.Spec = serviceSpec
	service.Name = cfg.CheckServiceName //+ "-" + strconv.Itoa(int(now.Unix()))
	service.Namespace = cfg.CheckNamespace

	return service
}
