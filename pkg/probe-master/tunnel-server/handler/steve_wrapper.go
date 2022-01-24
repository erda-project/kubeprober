package handler

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/erda-project/erda/apistructs"
	"github.com/erda-project/erda/modules/cmp/steve"
	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/pkg/probe-master/k8sclient"
	dialclient "github.com/erda-project/kubeprober/pkg/probe-master/tunnel-client"
)

type group struct {
	ready  bool
	server *steve.Server
	cancel context.CancelFunc
}

type Aggregator struct {
	Ctx     context.Context
	servers sync.Map
}

var a *Aggregator

// NewAggregator new an aggregator with steve servers for all current clusters
func NewAggregator(ctx context.Context) *Aggregator {
	a = &Aggregator{Ctx: ctx}
	a.init()
	go a.watchClusters(ctx)
	return a
}

func (a *Aggregator) init() {
	clusters, err := k8sclient.GetClusters()
	if err != nil {
		logrus.Errorf("failed to list clusters, %v", err)
		return
	}

	for i := range clusters {
		a.Add(clusters[i])
	}
}

// Add starts a steve server for k8s cluster with clusterName and add it into aggregator
func (a *Aggregator) Add(clusterInfo kubeproberv1.Cluster) {
	if _, ok := a.servers.Load(clusterInfo.Name); ok {
		return
	}

	g := &group{ready: false}
	a.servers.Store(clusterInfo.Name, g)

	// create steve server async
	go func() {
		logrus.Infof("starting steve server for cluster %s", clusterInfo.Name)
		server, cancel, err := a.createSteve(clusterInfo)
		if err != nil {
			logrus.Errorf("failed to create steve server for cluster %s, %v", clusterInfo.Name, err)
			a.servers.Delete(clusterInfo.Name)
			return
		}

		g = &group{
			ready:  true,
			server: server,
			cancel: cancel,
		}
		a.servers.Store(clusterInfo.Name, g)
		logrus.Infof("steve server for cluster %s started", clusterInfo.Name)
	}()
}

// Delete closes a steve server for k8s cluster with clusterName and delete it from aggregator
func (a *Aggregator) Delete(clusterName string) {
	g, ok := a.servers.Load(clusterName)
	if !ok {
		return
	}

	group, _ := g.(*group)
	if group.ready {
		group.cancel()
	}
	a.servers.Delete(clusterName)
}

func (a *Aggregator) watchClusters(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			clusters, err := k8sclient.GetClusters()
			if err != nil {
				logrus.Errorf("failed to list k8s clusters when watch: %v", err)
				continue
			}
			exists := make(map[string]struct{})
			for _, cluster := range clusters {
				exists[cluster.Name] = struct{}{}
				if _, ok := a.servers.Load(cluster.Name); ok {
					continue
				}
				a.Add(cluster)
			}

			checkDeleted := func(key interface{}, value interface{}) (res bool) {
				res = true
				if _, ok := exists[key.(string)]; ok {
					return
				}
				a.Delete(key.(string))
				return
			}
			a.servers.Range(checkDeleted)
		}
	}
}

func (a *Aggregator) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	clusterName := vars["clusterName"]

	if clusterName == "" {
		rw.WriteHeader(http.StatusNotFound)
		rw.Write(apistructs.NewSteveError(apistructs.NotFound, "cluster name is required").JSON())
		return
	}

	s, ok := a.servers.Load(clusterName)
	if !ok {
		cluster, err := k8sclient.GetCluster(clusterName)
		if err != nil {
			logrus.Errorf("failed to get cluster %s, %v", clusterName, err)
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write(apistructs.NewSteveError(apistructs.ServerError, "Internal server error").JSON())
			return
		}
		if cluster == nil {
			rw.WriteHeader(http.StatusNotFound)
			rw.Write(apistructs.NewSteveError(apistructs.NotFound,
				fmt.Sprintf("cluster %s not found", clusterName)).JSON())
			return
		}

		logrus.Infof("steve for cluster %s not exist, starting a new server", cluster.Name)
		a.Add(*cluster)
		if s, ok = a.servers.Load(cluster.Name); !ok {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write(apistructs.NewSteveError(apistructs.ServerError, "Internal server error").JSON())
			return
		}
	}

	group, _ := s.(*group)
	if !group.ready {
		logrus.Errorf("steve for cluster %s not ready", clusterName)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write(apistructs.NewSteveError(apistructs.ServerError,
			fmt.Sprintf("API for cluster %s is not ready, please wait", clusterName)).JSON())
		return
	}

	group.server.ServeHTTP(rw, req)
}

func (a *Aggregator) createSteve(clusterInfo kubeproberv1.Cluster) (*steve.Server, context.CancelFunc, error) {
	restConfig, err := dialclient.GenerateProbeClientConf(&clusterInfo)
	if err != nil {
		logrus.Errorf("failed to create rest config, %v", err)
		return nil, nil, err
	}

	ctx, cancel := context.WithCancel(a.Ctx)
	prefix := steve.GetURLPrefix(clusterInfo.Name)
	server, err := steve.New(ctx, restConfig, &steve.Options{
		Router:      steve.RoutesWrapper(prefix),
		URLPrefix:   prefix,
		ClusterName: clusterInfo.Name,
	})
	if err != nil {
		logrus.Errorf("failed to new steve server for cluster %s, %v", clusterInfo.Name, err)
		cancel()
		return nil, nil, err
	}

	return server, cancel, nil
}
