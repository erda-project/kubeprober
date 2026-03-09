package v1

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestClusterWebhookRejectsUnexpectedObjects(t *testing.T) {
	ctx := context.Background()
	cluster := &Cluster{}
	wrong := &Probe{}

	if err := cluster.Default(ctx, wrong); err == nil {
		t.Fatalf("expected cluster default to reject unexpected object")
	}
	if _, err := cluster.ValidateCreate(ctx, wrong); err == nil {
		t.Fatalf("expected cluster validate create to reject unexpected object")
	}
	if _, err := cluster.ValidateUpdate(ctx, &Cluster{}, wrong); err == nil {
		t.Fatalf("expected cluster validate update to reject unexpected object")
	}
	if _, err := cluster.ValidateDelete(ctx, wrong); err == nil {
		t.Fatalf("expected cluster validate delete to reject unexpected object")
	}
}

func TestProbeWebhookRejectsUnexpectedObjects(t *testing.T) {
	ctx := context.Background()
	probe := &Probe{}
	wrong := &Cluster{}

	if err := probe.Default(ctx, wrong); err == nil {
		t.Fatalf("expected probe default to reject unexpected object")
	}
	if _, err := probe.ValidateCreate(ctx, wrong); err == nil {
		t.Fatalf("expected probe validate create to reject unexpected object")
	}
	if _, err := probe.ValidateUpdate(ctx, &Probe{}, wrong); err == nil {
		t.Fatalf("expected probe validate update to reject unexpected object")
	}
	if _, err := probe.ValidateDelete(ctx, wrong); err == nil {
		t.Fatalf("expected probe validate delete to reject unexpected object")
	}
}

func TestClusterWebhookAcceptsClusterObjects(t *testing.T) {
	ctx := context.Background()
	cluster := &Cluster{}
	obj := runtime.Object(&Cluster{})

	if err := cluster.Default(ctx, obj); err != nil {
		t.Fatalf("unexpected default error: %v", err)
	}
	if _, err := cluster.ValidateCreate(ctx, obj); err != nil {
		t.Fatalf("unexpected validate create error: %v", err)
	}
	if _, err := cluster.ValidateUpdate(ctx, &Cluster{}, obj); err != nil {
		t.Fatalf("unexpected validate update error: %v", err)
	}
	if _, err := cluster.ValidateDelete(ctx, obj); err != nil {
		t.Fatalf("unexpected validate delete error: %v", err)
	}
}

func TestProbeWebhookAcceptsProbeObjects(t *testing.T) {
	ctx := context.Background()
	probe := &Probe{}
	obj := runtime.Object(&Probe{})

	if err := probe.Default(ctx, obj); err != nil {
		t.Fatalf("unexpected default error: %v", err)
	}
	if _, err := probe.ValidateCreate(ctx, obj); err != nil {
		t.Fatalf("unexpected validate create error: %v", err)
	}
	if _, err := probe.ValidateUpdate(ctx, &Probe{}, obj); err != nil {
		t.Fatalf("unexpected validate update error: %v", err)
	}
}
