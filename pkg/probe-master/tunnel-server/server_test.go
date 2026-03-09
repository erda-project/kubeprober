package server

import (
	"net/http/httptest"
	"testing"
)

func TestNotifyClusterReadyEnqueuesCluster(t *testing.T) {
	t.Cleanup(func() {
		enqueueClusterReconcile = defaultEnqueueClusterReconcile
	})

	var got string
	enqueueClusterReconcile = func(clusterName string) {
		got = clusterName
	}

	notifyClusterReady(" ash-test ")

	if got != "ash-test" {
		t.Fatalf("expected ash-test, got %q", got)
	}
}

func TestNotifyClusterConnectEnqueuesAuthedCluster(t *testing.T) {
	t.Setenv("SERVER_SECRET_KEY", "secret")
	t.Cleanup(func() {
		enqueueClusterReconcile = defaultEnqueueClusterReconcile
	})

	var got string
	enqueueClusterReconcile = func(clusterName string) {
		got = clusterName
	}

	req := httptest.NewRequest("GET", "/clusteragent/connect", nil)
	req.Header.Set("X-Cluster-Name", "ash-test")
	req.Header.Set("Secret-Key", "secret")

	notifyClusterConnect(req)

	if got != "ash-test" {
		t.Fatalf("expected ash-test, got %q", got)
	}
}

func TestNotifyClusterConnectSkipsUnauthedCluster(t *testing.T) {
	t.Setenv("SERVER_SECRET_KEY", "secret")
	t.Cleanup(func() {
		enqueueClusterReconcile = defaultEnqueueClusterReconcile
	})

	called := false
	enqueueClusterReconcile = func(clusterName string) {
		called = true
	}

	req := httptest.NewRequest("GET", "/clusteragent/connect", nil)
	req.Header.Set("X-Cluster-Name", "ash-test")
	req.Header.Set("Secret-Key", "wrong")

	notifyClusterConnect(req)

	if called {
		t.Fatalf("expected unauthenticated request to be ignored")
	}
}
