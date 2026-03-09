package controller

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/event"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
)

const defaultClusterEventBufferSize = 128

type ClusterEventSource struct {
	events chan event.GenericEvent
}

func NewClusterEventSource(bufferSize int) *ClusterEventSource {
	if bufferSize <= 0 {
		bufferSize = defaultClusterEventBufferSize
	}
	return &ClusterEventSource{
		events: make(chan event.GenericEvent, bufferSize),
	}
}

func (s *ClusterEventSource) Events() <-chan event.GenericEvent {
	return s.events
}

func (s *ClusterEventSource) Enqueue(clusterName string) bool {
	if s == nil || clusterName == "" {
		return false
	}

	evt := event.GenericEvent{
		Object: &kubeproberv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName,
				Namespace: metav1.NamespaceDefault,
			},
		},
	}

	select {
	case s.events <- evt:
		return true
	default:
		klog.V(1).Infof("drop cluster reconcile event for %s because event queue is full", clusterName)
		return false
	}
}

var (
	clusterEventSourceOnce sync.Once
	clusterEventSource     *ClusterEventSource
)

func GetClusterEventSource() *ClusterEventSource {
	clusterEventSourceOnce.Do(func() {
		clusterEventSource = NewClusterEventSource(defaultClusterEventBufferSize)
	})
	return clusterEventSource
}
