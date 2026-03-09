package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
)

func TestClusterEventSourceEnqueue(t *testing.T) {
	source := NewClusterEventSource(1)
	if ok := source.Enqueue("ash-test"); !ok {
		t.Fatalf("expected enqueue to succeed")
	}

	select {
	case evt := <-source.Events():
		cluster, ok := evt.Object.(*kubeproberv1.Cluster)
		if !ok {
			t.Fatalf("expected cluster object, got %T", evt.Object)
		}
		if cluster.Namespace != metav1.NamespaceDefault {
			t.Fatalf("unexpected namespace: %s", cluster.Namespace)
		}
		if cluster.Name != "ash-test" {
			t.Fatalf("unexpected cluster name: %s", cluster.Name)
		}
	default:
		t.Fatalf("expected a generic event to be emitted")
	}
}

func TestClusterPredicateCreateIgnored(t *testing.T) {
	predicate := &ClusterPredicate{}
	cluster := &kubeproberv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ash-test",
			Namespace: metav1.NamespaceDefault,
		},
	}

	if predicate.Create(event.CreateEvent{Object: cluster}) {
		t.Fatalf("expected create events to be ignored to avoid startup blind scan")
	}
}
