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

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/pkg/kubeclient"
	"github.com/erda-project/kubeprober/pkg/probe-master/k8sclient"
)

const (
	defaultSocksPortStart = 20000
	defaultSocksPortEnd   = 30000

	defaultSocksServicePort  = 1080
	socksServicePortName     = "socks5"
	socksServiceGenerateName = "kubeprober-socks5-"
)

type SocksManager struct {
	ctx              context.Context
	bindHost         string
	serviceNamespace string

	mu      sync.Mutex
	servers map[string]*socksServer
	pool    *portPool
}

func NewSocksManager(ctx context.Context, listenAddr string) *SocksManager {
	manager := &SocksManager{
		ctx:              ctx,
		bindHost:         parseBindHost(listenAddr),
		serviceNamespace: kubeclient.ResolveServiceNamespace(),
		servers:          make(map[string]*socksServer),
		pool:             newPortPool(defaultSocksPortStart, defaultSocksPortEnd),
	}
	go manager.watchClusters()
	return manager
}

func (m *SocksManager) watchClusters() {
	defer m.stopAll()

	if k8sclient.RestConfig == nil {
		logrus.Errorf("socks5 manager cannot watch clusters: rest config is nil")
		return
	}

	logrus.Infof("socks5 watch started for all namespaces")

	dynClient, err := dynamic.NewForConfig(k8sclient.RestConfig)
	if err != nil {
		logrus.Errorf("failed to create dynamic client for socks5 manager: %v", err)
		return
	}

	gvr := schema.GroupVersionResource{
		Group:    kubeproberv1.GroupVersion.Group,
		Version:  kubeproberv1.GroupVersion.Version,
		Resource: "clusters",
	}

	listWatch := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return dynClient.Resource(gvr).Namespace(metav1.NamespaceAll).List(m.ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return dynClient.Resource(gvr).Namespace(metav1.NamespaceAll).Watch(m.ctx, options)
		},
	}

	informer := cache.NewSharedIndexInformer(listWatch, &unstructured.Unstructured{}, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    m.handleClusterEvent,
		UpdateFunc: func(_, newObj interface{}) { m.handleClusterEvent(newObj) },
		DeleteFunc: m.handleClusterDelete,
	})

	informer.Run(m.ctx.Done())
	logrus.Infof("socks5 watch stopped")
}

func (m *SocksManager) ensureRunning(cluster *kubeproberv1.Cluster, desiredPort int, desiredPortSet bool) {
	var existing *socksServer

	clusterNS := clusterNamespace(cluster)
	clusterID := clusterKey(clusterNS, cluster.Name)

	m.mu.Lock()
	existing = m.servers[clusterID]
	if existing != nil && desiredPortSet && existing.port != desiredPort {
		m.stopLocked(clusterID)
		existing = nil
	}
	m.mu.Unlock()

	if existing != nil {
		m.ensureService(cluster, existing.port)
		m.maybeUpdateBoundPort(cluster, existing.port)
		return
	}

	entry, err := m.startServer(clusterID, cluster.Name, desiredPort, desiredPortSet)
	if err != nil {
		logrus.Errorf("failed to start socks5 for cluster %s: %v", clusterID, err)
		return
	}

	m.mu.Lock()
	m.servers[clusterID] = entry
	m.mu.Unlock()

	m.ensureService(cluster, entry.port)
	logrus.Infof("socks5 server started for cluster %s on port %d", clusterID, entry.port)
	m.maybeUpdateBoundPort(cluster, entry.port)
}

func (m *SocksManager) ensureStopped(cluster *kubeproberv1.Cluster) {
	clusterNS := clusterNamespace(cluster)
	clusterID := clusterKey(clusterNS, cluster.Name)
	m.stop(clusterID)
	m.deleteService(cluster)
	m.clearBoundPort(clusterNS, cluster.Name)
	m.clearServiceName(clusterNS, cluster.Name)
}

func (m *SocksManager) stopAll() {
	m.mu.Lock()
	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	m.mu.Unlock()

	for _, name := range names {
		m.stop(name)
	}
}

func (m *SocksManager) stop(name string) bool {
	m.mu.Lock()
	stopped := m.stopLocked(name)
	m.mu.Unlock()
	return stopped
}

func (m *SocksManager) stopLocked(name string) bool {
	entry, ok := m.servers[name]
	if !ok {
		return false
	}

	delete(m.servers, name)
	m.pool.release(entry.port)
	entry.listener.Close()
	logrus.Infof("socks5 server stopped for cluster %s on port %d", name, entry.port)
	return true
}

func (m *SocksManager) startServer(clusterKey, dialerKey string, desiredPort int, desiredPortSet bool) (*socksServer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	port, listener, err := m.allocateListener(clusterKey, desiredPort, desiredPortSet)
	if err != nil {
		return nil, err
	}

	server, err := newSocksServer(clusterKey, dialerKey, port, listener)
	if err != nil {
		listener.Close()
		m.pool.release(port)
		return nil, err
	}

	go m.serve(server)

	return server, nil
}

func (m *SocksManager) serve(entry *socksServer) {
	err := entry.Serve()
	if err != nil && !isClosedError(err) {
		logrus.Errorf("socks5 server for cluster %s stopped: %v", entry.clusterKey, err)
	}

	m.mu.Lock()
	if current, ok := m.servers[entry.clusterKey]; ok && current == entry {
		delete(m.servers, entry.clusterKey)
		m.pool.release(entry.port)
	}
	m.mu.Unlock()
}

func (m *SocksManager) allocateListener(owner string, desiredPort int, desiredPortSet bool) (int, net.Listener, error) {
	if desiredPortSet {
		if !m.pool.reserve(owner, desiredPort) {
			return 0, nil, fmt.Errorf("port %d already in use", desiredPort)
		}
		listener, err := m.listen(desiredPort)
		if err != nil {
			m.pool.release(desiredPort)
			return 0, nil, err
		}
		return desiredPort, listener, nil
	}

	for port := m.pool.start; port <= m.pool.end; port++ {
		if !m.pool.reserve(owner, port) {
			continue
		}
		listener, err := m.listen(port)
		if err != nil {
			m.pool.release(port)
			continue
		}
		return port, listener, nil
	}

	return 0, nil, fmt.Errorf("no available ports in range %d-%d", m.pool.start, m.pool.end)
}

func (m *SocksManager) listen(port int) (net.Listener, error) {
	addr := net.JoinHostPort(m.bindHost, strconv.Itoa(port))
	return net.Listen("tcp", addr)
}

func (m *SocksManager) maybeUpdateBoundPort(cluster *kubeproberv1.Cluster, port int) {
	current := ""
	if cluster.Annotations != nil {
		current = cluster.Annotations[kubeproberv1.AnnotationSocks5BoundPort]
	}
	desired := strconv.Itoa(port)
	if current == desired {
		return
	}
	clusterNS := clusterNamespace(cluster)
	if err := patchClusterAnnotation(clusterNS, cluster.Name, kubeproberv1.AnnotationSocks5BoundPort, desired); err != nil {
		logrus.Errorf("failed to update socks5 bound port for cluster %s/%s: %v", clusterNS, cluster.Name, err)
	}
}

func (m *SocksManager) clearBoundPort(clusterNamespace, clusterName string) {
	if err := patchClusterAnnotation(clusterNamespace, clusterName, kubeproberv1.AnnotationSocks5BoundPort, ""); err != nil {
		logrus.Errorf("failed to clear socks5 bound port for cluster %s/%s: %v", clusterNamespace, clusterName, err)
	}
}

func (m *SocksManager) handleClusterEvent(obj interface{}) {
	cluster, ok := clusterFromObject(obj)
	if !ok {
		return
	}

	if cluster.Annotations[kubeproberv1.AnnotationSocks5Enable] != "true" {
		m.ensureStopped(cluster)
		return
	}

	port, portSet := parsePort(cluster.Annotations[kubeproberv1.AnnotationSocks5Port])
	if !portSet {
		if bound, boundSet := parsePort(cluster.Annotations[kubeproberv1.AnnotationSocks5BoundPort]); boundSet {
			port = bound
			portSet = true
		}
	}

	m.ensureRunning(cluster, port, portSet)
}

func (m *SocksManager) handleClusterDelete(obj interface{}) {
	cluster, ok := clusterFromObject(obj)
	if !ok {
		return
	}
	clusterNS := clusterNamespace(cluster)
	clusterID := clusterKey(clusterNS, cluster.Name)
	m.stop(clusterID)
	m.deleteService(cluster)
}

func clusterFromObject(obj interface{}) (*kubeproberv1.Cluster, bool) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	switch typed := obj.(type) {
	case *kubeproberv1.Cluster:
		return typed, true
	case *unstructured.Unstructured:
		var cluster kubeproberv1.Cluster
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(typed.Object, &cluster)
		if err != nil {
			logrus.Errorf("failed to convert cluster object: %v", err)
			return nil, false
		}
		return &cluster, true
	default:
		return nil, false
	}
}

func clusterNamespace(cluster *kubeproberv1.Cluster) string {
	if cluster.Namespace == "" {
		return metav1.NamespaceDefault
	}
	return cluster.Namespace
}

func clusterKey(namespace, name string) string {
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}
	return namespace + "/" + name
}

func (m *SocksManager) ensureService(cluster *kubeproberv1.Cluster, targetPort int) {
	clusterName := cluster.Name
	clusterNS := clusterNamespace(cluster)
	clusterID := clusterKey(clusterNS, clusterName)
	serviceName := m.getServiceName(cluster)
	if serviceName == "" {
		serviceName = m.findServiceName(clusterName, clusterNS)
		if serviceName != "" {
			m.storeServiceName(clusterNS, clusterName, serviceName)
		}
	}
	desired := buildSocksService(clusterName, clusterNS, m.serviceNamespace, targetPort, serviceName)

	existing := &corev1.Service{}
	if serviceName != "" {
		err := k8sclient.RestClient.Get(m.ctx, types.NamespacedName{
			Name:      serviceName,
			Namespace: m.serviceNamespace,
		}, existing)
		if err != nil {
			if apierrors.IsNotFound(err) {
				if err := k8sclient.RestClient.Create(m.ctx, desired); err != nil {
					logrus.Errorf("failed to recreate socks5 service for cluster %s: %v", clusterID, err)
					return
				}
				logrus.Infof("socks5 service recreated for cluster %s", clusterID)
				return
			}
			logrus.Errorf("failed to get socks5 service for cluster %s: %v", clusterID, err)
			return
		}
	} else {
		if err := k8sclient.RestClient.Create(m.ctx, desired); err != nil {
			logrus.Errorf("failed to create socks5 service for cluster %s: %v", clusterID, err)
			return
		}
		logrus.Infof("socks5 service created for cluster %s", clusterID)
		m.storeServiceName(clusterNS, clusterName, desired.Name)
		return
	}

	updated := existing.DeepCopy()
	updated.Labels = desired.Labels
	updated.Spec.Ports = desired.Spec.Ports
	updated.Spec.Selector = desired.Spec.Selector
	updated.Spec.Type = desired.Spec.Type
	updated.Spec.SessionAffinity = desired.Spec.SessionAffinity
	updated.Spec.ClusterIP = existing.Spec.ClusterIP
	updated.Spec.ClusterIPs = existing.Spec.ClusterIPs

	if equality.Semantic.DeepEqual(existing.Spec, updated.Spec) && equality.Semantic.DeepEqual(existing.Labels, updated.Labels) {
		return
	}

	if err := k8sclient.RestClient.Update(m.ctx, updated); err != nil {
		logrus.Errorf("failed to update socks5 service for cluster %s: %v", clusterID, err)
		return
	}
	logrus.Infof("socks5 service updated for cluster %s", clusterID)
}

func (m *SocksManager) findServiceName(clusterName, clusterNamespace string) string {
	clusterID := clusterKey(clusterNamespace, clusterName)
	serviceList := &corev1.ServiceList{}
	err := k8sclient.RestClient.List(m.ctx, serviceList,
		client.InNamespace(m.serviceNamespace),
		client.MatchingLabels{
			kubeproberv1.LabelKeyCluster:   clusterName,
			kubeproberv1.LabelKeyClusterNS: clusterNamespace,
			kubeproberv1.LabelKeyService:   kubeproberv1.LabelValueServiceSocks5,
		},
	)
	if err != nil {
		logrus.Errorf("failed to list socks5 services for cluster %s: %v", clusterID, err)
		return ""
	}
	if len(serviceList.Items) == 0 {
		return ""
	}
	if len(serviceList.Items) > 1 {
		logrus.Warnf("multiple socks5 services found for cluster %s, using %s", clusterID, serviceList.Items[0].Name)
	}
	return serviceList.Items[0].Name
}

func (m *SocksManager) deleteService(cluster *kubeproberv1.Cluster) {
	clusterName := cluster.Name
	clusterNS := clusterNamespace(cluster)
	clusterID := clusterKey(clusterNS, clusterName)
	serviceName := m.getServiceName(cluster)
	if serviceName != "" {
		err := k8sclient.RestClient.Delete(m.ctx, &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: m.serviceNamespace,
			},
		})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return
			}
			logrus.Errorf("failed to delete socks5 service for cluster %s: %v", clusterID, err)
			return
		}
		logrus.Infof("socks5 service deleted for cluster %s", clusterID)
		return
	}

	serviceList := &corev1.ServiceList{}
	err := k8sclient.RestClient.List(m.ctx, serviceList,
		client.InNamespace(m.serviceNamespace),
		client.MatchingLabels{
			kubeproberv1.LabelKeyCluster:   clusterName,
			kubeproberv1.LabelKeyClusterNS: clusterNS,
			kubeproberv1.LabelKeyService:   kubeproberv1.LabelValueServiceSocks5,
		},
	)
	if err != nil {
		logrus.Errorf("failed to list socks5 services for cluster %s: %v", clusterID, err)
		return
	}
	for i := range serviceList.Items {
		svc := serviceList.Items[i]
		if err := k8sclient.RestClient.Delete(m.ctx, &svc); err != nil && !apierrors.IsNotFound(err) {
			logrus.Errorf("failed to delete socks5 service %s for cluster %s: %v", svc.Name, clusterID, err)
		}
	}
}

func buildSocksService(clusterName, clusterNamespace, namespace string, targetPort int, serviceName string) *corev1.Service {
	meta := metav1.ObjectMeta{
		Namespace: namespace,
		Labels: map[string]string{
			kubeproberv1.LabelKeyApp:       kubeproberv1.LabelValueProbeMaster,
			kubeproberv1.LabelKeyCluster:   clusterName,
			kubeproberv1.LabelKeyClusterNS: clusterNamespace,
			kubeproberv1.LabelKeyService:   kubeproberv1.LabelValueServiceSocks5,
		},
	}
	if serviceName == "" {
		meta.GenerateName = socksServiceGenerateName
	} else {
		meta.Name = serviceName
	}

	return &corev1.Service{
		ObjectMeta: meta,
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       socksServicePortName,
					Port:       defaultSocksServicePort,
					TargetPort: intstr.FromInt(targetPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				kubeproberv1.LabelKeyApp: kubeproberv1.LabelValueProbeMaster,
			},
			SessionAffinity: corev1.ServiceAffinityNone,
		},
	}
}

func (m *SocksManager) getServiceName(cluster *kubeproberv1.Cluster) string {
	if cluster.Annotations == nil {
		return ""
	}
	return strings.TrimSpace(cluster.Annotations[kubeproberv1.AnnotationSocks5ServiceName])
}

func (m *SocksManager) storeServiceName(clusterNamespace, clusterName, name string) {
	if name == "" {
		return
	}
	if err := patchClusterAnnotation(clusterNamespace, clusterName, kubeproberv1.AnnotationSocks5ServiceName, name); err != nil {
		logrus.Errorf("failed to store socks5 service name for cluster %s/%s: %v", clusterNamespace, clusterName, err)
	}
}

func (m *SocksManager) clearServiceName(clusterNamespace, clusterName string) {
	if err := patchClusterAnnotation(clusterNamespace, clusterName, kubeproberv1.AnnotationSocks5ServiceName, ""); err != nil {
		logrus.Errorf("failed to clear socks5 service name for cluster %s/%s: %v", clusterNamespace, clusterName, err)
	}
}

func parseBindHost(listenAddr string) string {
	if listenAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(listenAddr)
	if err == nil {
		return host
	}
	if strings.Contains(listenAddr, ":") {
		parts := strings.Split(listenAddr, ":")
		return parts[0]
	}
	return listenAddr
}

func parsePort(value string) (int, bool) {
	if strings.TrimSpace(value) == "" {
		return 0, false
	}
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port <= 0 || port > 65535 {
		return 0, false
	}
	return port, true
}

func patchClusterAnnotation(clusterNamespace, clusterName, key, value string) error {
	if clusterNamespace == "" {
		clusterNamespace = metav1.NamespaceDefault
	}
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				key: value,
			},
		},
	}

	if value == "" {
		patch["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})[key] = nil
	}

	body, err := json.Marshal(patch)
	if err != nil {
		return err
	}

	return k8sclient.RestClient.Patch(context.Background(), &kubeproberv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: clusterNamespace,
		},
	}, client.RawPatch(types.MergePatchType, body))
}

func isClosedError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	return utilnet.IsProbableEOF(err)
}

type portPool struct {
	start int
	end   int
	used  map[int]string
}

func newPortPool(start, end int) *portPool {
	return &portPool{
		start: start,
		end:   end,
		used:  make(map[int]string),
	}
}

func (p *portPool) reserve(owner string, port int) bool {
	if current, ok := p.used[port]; ok && current != owner {
		return false
	}
	p.used[port] = owner
	return true
}

func (p *portPool) release(port int) {
	delete(p.used, port)
}
