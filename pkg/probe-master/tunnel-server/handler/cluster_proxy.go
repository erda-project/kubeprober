package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/pkg/probe-master/k8sclient"
	dialclient "github.com/erda-project/kubeprober/pkg/probe-master/tunnel-client"
)

type ProxyManager struct {
	Ctx context.Context
}

var proxyManager *ProxyManager

// NewProxyManager initializes the proxy manager used by console/exec proxying.
func NewProxyManager(ctx context.Context) *ProxyManager {
	proxyManager = &ProxyManager{Ctx: ctx}
	return proxyManager
}

func (p *ProxyManager) ProxyRequest(rw http.ResponseWriter, req *http.Request, clusterName string) {
	if clusterName == "" {
		http.Error(rw, "cluster name is required", http.StatusNotFound)
		return
	}

	cluster, err := k8sclient.GetCluster(clusterName)
	if err != nil {
		logrus.Errorf("failed to get cluster %s, %v", clusterName, err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}
	if cluster == nil {
		http.Error(rw, fmt.Sprintf("cluster %s not found", clusterName), http.StatusNotFound)
		return
	}

	handler, err := newClusterProxyHandler(cluster)
	if err != nil {
		logrus.Errorf("failed to create proxy handler for cluster %s, %v", clusterName, err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}

	handler.ServeHTTP(rw, req)
}

func newClusterProxyHandler(cluster *kubeproberv1.Cluster) (http.Handler, error) {
	restConfig, err := dialclient.GenerateProbeClientConf(cluster)
	if err != nil {
		return nil, err
	}

	return newK8sProxyHandler(cluster.Name, restConfig)
}

func newK8sProxyHandler(clusterName string, cfg *rest.Config) (http.Handler, error) {
	host := cfg.Host
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}
	if !strings.HasSuffix(host, "/") {
		host = host + "/"
	}
	target, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	transportRT, err := rest.TransportFor(cfg)
	if err != nil {
		return nil, err
	}

	upgradeTransport, err := makeUpgradeTransport(cfg, transportRT)
	if err != nil {
		return nil, err
	}

	proxyHandler := proxy.NewUpgradeAwareHandler(target, transportRT, false, false, &proxyResponder{})
	proxyHandler.UpgradeTransport = upgradeTransport
	proxyHandler.UseRequestLocation = true
	proxyHandler.UseLocationHost = true

	handler := http.Handler(proxyHandler)
	if len(target.Path) > 1 {
		handler = prependPath(target.Path[:len(target.Path)-1], handler)
	}

	prefix := clusterURLPrefix(clusterName)
	if len(prefix) > 2 {
		handler = stripLeaveSlash(prefix, handler)
	}

	return proxyHeaders(handler), nil
}

type proxyResponder struct{}

func (r *proxyResponder) Error(w http.ResponseWriter, req *http.Request, err error) {
	logrus.Errorf("error while proxying request: %v", err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func clusterURLPrefix(clusterName string) string {
	return "/api/k8s/clusters/" + clusterName
}

func proxyHeaders(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		req.Header.Del("Authorization")
		if req.Header.Get("X-Forwarded-Proto") == "" && req.TLS != nil {
			req.Header.Set("X-Forwarded-Proto", "https")
		}
		handler.ServeHTTP(rw, req)
	})
}

func prependPath(prefix string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if len(req.URL.Path) > 1 {
			req.URL.Path = prefix + req.URL.Path
		} else {
			req.URL.Path = prefix
		}
		handler.ServeHTTP(rw, req)
	})
}

func stripLeaveSlash(prefix string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		path := strings.TrimPrefix(req.URL.Path, prefix)
		if len(path) > 0 && path[:1] != "/" {
			path = "/" + path
		}
		req.URL.Path = path
		handler.ServeHTTP(rw, req)
	})
}

func makeUpgradeTransport(cfg *rest.Config, rt http.RoundTripper) (proxy.UpgradeRequestRoundTripper, error) {
	transportConfig, err := cfg.TransportConfig()
	if err != nil {
		return nil, err
	}

	upgrader, err := transport.HTTPWrappersForConfig(transportConfig, proxy.MirrorRequest)
	if err != nil {
		return nil, err
	}

	return proxy.NewUpgradeRequestRoundTripper(rt, upgrader), nil
}
