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

package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2api "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/pkg/errors"
	"github.com/rancher/remotedialer"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	erda_api "github.com/erda-project/erda/apistructs"
	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/apistructs"
	"github.com/erda-project/kubeprober/pkg/probe-master/alert/dingding"
	"github.com/erda-project/kubeprober/pkg/probe-master/alert/ticket"
	"github.com/erda-project/kubeprober/pkg/probe-master/k8sclient"
	_ "github.com/erda-project/kubeprober/pkg/probe-master/k8sclient"
	httphandler "github.com/erda-project/kubeprober/pkg/probe-master/tunnel-server/handler"
)

func clusterRegister(server *remotedialer.Server, rw http.ResponseWriter, req *http.Request) {
	server.ServeHTTP(rw, req)
}

func heartbeat(rw http.ResponseWriter, req *http.Request) {
	hbData := apistructs.HeartBeatReq{}
	cluster := &kubeproberv1.Cluster{}
	var err error

	if err := json.NewDecoder(req.Body).Decode(&hbData); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	clusterSpec := kubeproberv1.Cluster{
		Spec: kubeproberv1.ClusterSpec{
			K8sVersion: hbData.Version,
			ClusterConfig: kubeproberv1.ClusterConfig{
				Address:         hbData.Address,
				Token:           hbData.Token,
				CACert:          hbData.CaData,
				CertData:        hbData.CertData,
				KeyData:         hbData.KeyData,
				ProbeNamespaces: hbData.ProbeNamespace,
			},
		},
	}
	err = k8sclient.RestClient.Get(context.Background(), client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      hbData.Name,
	}, cluster)
	if apierrors.IsNotFound(err) {
		clusterSpec.ObjectMeta = metav1.ObjectMeta{
			Name:      hbData.Name,
			Namespace: metav1.NamespaceDefault,
		}
		if err = k8sclient.RestClient.Create(context.Background(), &clusterSpec); err != nil {
			errMsg := fmt.Sprintf("[heartbeat] failed to create cluster [%s]: %+v\n", hbData.Name, err)
			rw.Write([]byte(errMsg))
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else if err != nil {
		errMsg := fmt.Sprintf("[heartbeat] failed to check cluster existence [%s]: %+v\n", hbData.Name, err)
		rw.Write([]byte(errMsg))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	} else {
		patch, _ := json.Marshal(clusterSpec)
		err = k8sclient.RestClient.Patch(context.Background(), &kubeproberv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hbData.Name,
				Namespace: metav1.NamespaceDefault,
			},
		}, client.RawPatch(types.MergePatchType, patch))
		if err != nil {
			errMsg := fmt.Sprintf("[heartbeat] patch cluster[%s] spec error: %+v\n", hbData.Name, err)
			klog.Errorf(errMsg)
			rw.Write([]byte(errMsg))
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	statusPatchBody := kubeproberv1.Cluster{
		Status: kubeproberv1.ClusterStatus{
			HeartBeatTimeStamp: time.Now().In(loc).Format("2006-01-02 15:04:05"),
			NodeCount:          hbData.NodeCount,
			Checkers:           hbData.Checkers,
			ExtraStatus:        hbData.ExtraStatus,
		},
	}
	statusPatch, _ := json.Marshal(statusPatchBody)
	err = k8sclient.RestClient.Status().Patch(context.Background(), &kubeproberv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hbData.Name,
			Namespace: metav1.NamespaceDefault,
		},
	}, client.RawPatch(types.MergePatchType, statusPatch))
	if err != nil {
		errMsg := fmt.Sprintf("[heartbeat] patch cluster[%s] status error: %+v\n", hbData.Name, err)
		klog.Errorf(errMsg)
		rw.Write([]byte(errMsg))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = updateExternalPrometheusConfigMap(hbData.Name)
	if err != nil {
		errMsg := fmt.Sprintf("[heartbeat] update configmap error for cluster[%s]: %+v\n", hbData.Name, err)
		klog.Errorf(errMsg)
		rw.Write([]byte(errMsg))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	rw.WriteHeader(http.StatusOK)
	return
}

func newDatasource(clusterName string) (string, error) {
	type Datasource struct {
		Name       string `yaml:"name"`
		Version    int    `yaml:"version"`
		Type       string `yaml:"type"`
		Url        string `yaml:"url"`
		HttpMethod string `yaml:"httpMethod"`
		Editable   bool   `yaml:"editable"`
	}
	type Config struct {
		APIVersion  int          `yaml:"apiVersion"`
		Datasources []Datasource `yaml:"datasources"`
	}

	dataName := fmt.Sprintf("external_%v", clusterName)
	cfg := Config{
		APIVersion: 1,
		Datasources: []Datasource{
			{
				Name:       dataName,
				Version:    1,
				Type:       "prometheus",
				Url:        fmt.Sprintf("http:///probe-master.kubeprober.svc.cluster.local:8088/%v/prometheus-bypass.erda-monitoring:9090", clusterName),
				HttpMethod: "post",
				Editable:   true,
			},
		},
	}

	cfgBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config for %v: %w", clusterName, err)
	}

	return string(cfgBytes), nil
}

var configMapCache map[string]struct {
	ConfigMap *corev1.ConfigMap
	TimeStamp time.Time
}

// 5 minute
const cacheExpiration = 5 * time.Minute

func getConfigMap(clusterName string) (*corev1.ConfigMap, error) {
	configMapName := "grafana-datasource"

	// check cache expire
	if cacheEntry, ok := configMapCache[clusterName]; ok {
		if time.Since(cacheEntry.TimeStamp) < cacheExpiration {
			return cacheEntry.ConfigMap, nil
		}
	}

	configMap := &corev1.ConfigMap{}
	if err := k8sclient.RestClient.Get(context.Background(), client.ObjectKey{
		Name: configMapName,
	}, configMap); err != nil {
		err = errors.Wrap(err, fmt.Sprintf("get config %v info", configMapName))
		return nil, err
	}

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}

	dataName := fmt.Sprintf("external_%v", clusterName)
	dataFileName := fmt.Sprintf("%v.yaml", dataName)

	if _, ok := configMap.Data[dataFileName]; !ok {
		datasource, err := newDatasource(clusterName)
		if err != nil {
			err = errors.Wrap(err, fmt.Sprintf("create %v datasource is failed", clusterName))
			return nil, err
		}
		configMap.Data[dataFileName] = datasource
		err = k8sclient.RestClient.Update(context.Background(), configMap)
		if err != nil {
			err = errors.Wrap(err, fmt.Sprintf("update config map is failed: %v ", clusterName))
			return nil, err
		}
	}

	// 将获取的配置项添加到缓存中，包括时间戳
	configMapCache[clusterName] = struct {
		ConfigMap *corev1.ConfigMap
		TimeStamp time.Time
	}{ConfigMap: configMap, TimeStamp: time.Now()}

	return configMap, nil
}

func updateExternalPrometheusConfigMap(clusterName string) error {
	configMap, err := getConfigMap(clusterName)
	if err != nil {
		return errors.Wrap(err, "get config map is failed")
	}
	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}

	dataName := fmt.Sprintf("external_%v", clusterName)
	dataFileName := fmt.Sprintf("%v.yaml", dataName)

	if _, ok := configMap.Data[dataFileName]; !ok {
		datasource, err := newDatasource(clusterName)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("create %v datasource is failed", clusterName))
		}
		configMap.Data[dataFileName] = datasource
		err = k8sclient.RestClient.Update(context.Background(), configMap)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("update config map is failed: %v ", clusterName))
		}
	}
	return nil
}

func Start(ctx context.Context, cfg *Config, influxdbConfig *apistructs.InfluxdbConf, erdaConfig *apistructs.ErdaConfig) error {
	var err error
	var client influxdb2.Client
	var writeAPI influxdb2api.WriteAPI
	var alertDataWriteAPI influxdb2api.WriteAPI

	if influxdbConfig.InfluxdbEnable {
		client = influxdb2.NewClient(influxdbConfig.InfluxdbHost, influxdbConfig.InfluxdbToken)
		writeAPI = client.WriteAPI(influxdbConfig.InfluxdbOrg, influxdbConfig.InfluxdbBucket)
		alertDataWriteAPI = client.WriteAPI(influxdbConfig.InfluxdbOrg, influxdbConfig.AlertDataBucket)
		defer client.Close()
	}

	if erdaConfig.TicketEnable {
		err = ticket.Init(erdaConfig.Username, erdaConfig.Password, erdaConfig.OpenapiURL,
			erdaConfig.Org, erdaConfig.ProjectId)
		if err != nil {
			klog.Errorf("failed to connect erda: %+v\n", err)
		}
	}

	handler := remotedialer.New(Authorizer, remotedialer.DefaultErrorWriter)
	handler.ClientConnectAuthorizer = func(proto, address string) bool {
		if strings.HasSuffix(proto, "::tcp") {
			return true
		}
		if strings.HasSuffix(proto, "::unix") {
			return address == "/var/run/docker.sock"
		}
		if strings.HasSuffix(proto, "::npipe") {
			return address == "//./pipe/docker_engine"
		}
		return false
	}
	// TODO: support handler.AddPeer
	router := mux.NewRouter()
	router.Handle("/clusterdialer", handler)
	router.Path("/heartbeat").Methods(http.MethodPost).HandlerFunc(heartbeat)
	router.HandleFunc("/clusteragent/connect", func(rw http.ResponseWriter,
		req *http.Request) {
		clusterRegister(handler, rw, req)
	})

	router.HandleFunc("/robot/send", func(rw http.ResponseWriter,
		req *http.Request) {
		proxyDingdingAlert(rw, req, alertDataWriteAPI)
	})

	router.HandleFunc("/collect", func(rw http.ResponseWriter,
		req *http.Request) {
		collectProbeStatus(rw, req, writeAPI)
	})

	router.HandleFunc("/cluster", func(rw http.ResponseWriter,
		req *http.Request) {
		rw.WriteHeader(http.StatusOK)
	})

	router.HandleFunc("/cluster/query", func(rw http.ResponseWriter,
		req *http.Request) {
		httphandler.GetClusterList(rw, req)
	})

	router.HandleFunc("/cluster/search", func(rw http.ResponseWriter,
		req *http.Request) {
		json.NewEncoder(rw).Encode([]string{"NODECOUNT"})
	})

	httphandler.NewAggregator(ctx)
	router.HandleFunc("/api/k8s/clusters/{clusterName}", func(rw http.ResponseWriter,
		req *http.Request) {
		httphandler.ClusterConsole(rw, req)
	})

	router.HandleFunc("/alertstatistic", func(rw http.ResponseWriter,
		req *http.Request) {
		rw.WriteHeader(http.StatusOK)
	})

	router.HandleFunc("/alertstatistic/query", func(rw http.ResponseWriter,
		req *http.Request) {
		httphandler.GetAlertStatistic(rw, req)
	})

	router.HandleFunc("/alertstatistic/search", func(rw http.ResponseWriter,
		req *http.Request) {
		json.NewEncoder(rw).Encode([]string{})
	})
	router.HandleFunc("/collect/bypass", func(rw http.ResponseWriter,
		req *http.Request) {
		targetURL := "http://prometheus.erda-monitoring:9090/api/v1/write"

		username := "probemaster"
		password := cfg.BypassAuthPassword

		// check basic auth
		auth := req.Header.Get("Authorization")
		validAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
		if auth != validAuth {
			rw.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(rw, "Unauthorized", http.StatusUnauthorized)
			return
		}

		target, err := url.Parse(targetURL)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		req.Host = target.Host
		proxy.ServeHTTP(rw, req)
	})
	router.HandleFunc("/tunnel/{cluster}/{path:.*}", handlePrometheusBypass(cfg, handler))

	server := &http.Server{
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
		Addr:    cfg.Listen,
		Handler: router,
	}
	return server.ListenAndServe()
}

func handlePrometheusBypass(cfg *Config, remoteServer *remotedialer.Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract the cluster name and target from the URL path
		vars := mux.Vars(r)
		clusterName := vars["cluster"]
		path := vars["path"]

		timeout := r.URL.Query().Get("timeout")
		if timeout == "" {
			timeout = "15"
		}

		cluster := &kubeproberv1.Cluster{}
		if err := k8sclient.RestClient.Get(context.Background(), client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      clusterName,
		}, cluster); err != nil {
			err = errors.Wrap(err, fmt.Sprintf("get cluster %v info", cluster))
			logrus.Errorf(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		client := getClient(remoteServer, clusterName, timeout)
		if client == nil {
			err := errors.New(fmt.Sprintf("Get client for cluster %v failed", clusterName))
			logrus.Errorf(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		targetURL := fmt.Sprintf("http://%s", path)

		req, err := http.NewRequest(r.Method, targetURL, r.Body)
		if err != nil {
			err = errors.Wrap(err, fmt.Sprintf("create request for %v", targetURL))
			logrus.Errorf(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Copy headers from the original request to the new request
		for key, values := range r.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}

		// Perform basic authentication if Authorization header is present
		if username, password, ok := r.BasicAuth(); ok {
			req.SetBasicAuth(username, password)
		}

		// Start timer
		startTime := time.Now()

		// Perform the request using the client
		resp, err := client.Do(req)
		defer resp.Body.Close()

		if err != nil {
			err = errors.Wrap(err, fmt.Sprintf("perform request for %v", targetURL))
			logrus.Errorf(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if cfg.Debug {
			duration := time.Since(startTime)
			logrus.Println("exec: %v,duration: %v, response.code: %v", targetURL, duration, resp.StatusCode)
		}

		// Copy the response headers to the client
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		// Copy the response status code to the client
		w.WriteHeader(resp.StatusCode)

		// Copy the response body to the client
		if _, err := io.Copy(w, resp.Body); err != nil {
			err = errors.Wrap(err, fmt.Sprintf("copy response body for %v", targetURL))
			logrus.Errorf(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func getClient(remoteServer *remotedialer.Server, clientKey string, timeout string) *http.Client {
	dialer := remoteServer.Dialer(clientKey)
	if dialer == nil {
		return nil
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: dialer,
		},
	}

	if timeout != "" {
		t, err := strconv.Atoi(timeout)
		if err == nil {
			client.Timeout = time.Duration(t) * time.Second
		}
	}

	return client
}

func proxyDingdingAlert(rw http.ResponseWriter, req *http.Request, influxdb2api influxdb2api.WriteAPI) {
	var (
		ignore   bool
		alertStr string
	)
	//
	//// if enable black list, check the copy of request body
	//if alert != nil && len(alert.Spec.BlackList) > 0 {
	// get buffer
	buf, _ := ioutil.ReadAll(req.Body)
	// copy buffer & re-assign to request body
	newBd := ioutil.NopCloser(bytes.NewBuffer(buf))
	req.Body = newBd
	alertStr = string(buf)
	//}

	ignore = dingding.CheckBlacklist(alertStr)
	// return if ignore
	if ignore {
		return
	}

	klog.Infof("alert string: %+v\n", alertStr)
	asItem, err := dingding.ParseAlert(alertStr)
	if err == nil {
		if influxdb2api != nil && asItem.Status == dingding.AlertEmit {
			p := influxdb2.NewPointWithMeasurement("alert").
				AddTag("cluster", asItem.Cluster).
				AddTag("node", asItem.Node).
				AddTag("type", asItem.Type).
				AddTag("component", asItem.Component).
				AddTag("level", asItem.Level).
				AddField("msg", asItem.Msg).
				SetTime(time.Now())
			// Flush writes
			influxdb2api.WritePoint(p)
			influxdb2api.Flush()
		}

		level := strings.ToLower(asItem.Level)
		if level == "fatal" || level == "critical" || level == "warning" ||
			asItem.Status == dingding.AlertRecover {
			t := &ticket.Ticket{
				Labels: []string{asItem.Cluster, asItem.Node, asItem.Type, "告警"},
			}
			if asItem.Component != "" {
				t.Labels = append(t.Labels, asItem.Component)
			}
			if asItem.Status == dingding.AlertRecover {
				t.Kind = ticket.PassTicket
			} else { // asItem.Status == dingding.AlertRecover
				t.Kind = ticket.ErrorTicket
			}
			t.Title = fmt.Sprintf("(请勿改标题) 异常告警-[集群]: %s,[节点]: %s,[类别]: %s,[组件]: %s",
				asItem.Cluster, asItem.Node, asItem.Type, asItem.Component)
			t.Content = asItem.Msg
			t.Type = erda_api.IssueTypeTicket
			if level == "fatal" {
				t.Priority = erda_api.IssuePriorityUrgent
			} else if level == "critical" {
				t.Priority = erda_api.IssuePriorityHigh
			} else if level == "warning" {
				t.Priority = erda_api.IssuePriorityNormal
			}

			ticket.SendTicket(t)
		}
	}
	klog.Errorf("alert start send to dingding\n")
	dingding.ProxyAlert(rw, req)
}

func collectProbeStatus(rw http.ResponseWriter, req *http.Request, influxdb2api influxdb2api.WriteAPI) {
	ps := apistructs.CollectProbeStatusReq{}
	var err error
	if err = json.NewDecoder(req.Body).Decode(&ps); err != nil {
		errMsg := fmt.Sprintf("receive probe status err: %+v\n", err)
		klog.Errorf(errMsg)
		rw.WriteHeader(http.StatusBadRequest)
		rw.Write([]byte(errMsg))
		return
	}
	if influxdb2api != nil {
		//influxdb2api.WriteRecord(fmt.Sprintf("checker,cluster=%s,checker=%s,probe=%s result=\"%s###%s\"", ps.ClusterName, ps.CheckerName, ps.ProbeName, ps.Status, ps.Message))
		p := influxdb2.NewPointWithMeasurement("checker").
			AddTag("cluster", ps.ClusterName).
			AddTag("checker", ps.CheckerName).
			AddTag("probe", ps.ProbeName).
			AddField("result", fmt.Sprintf("%s###%s", ps.Status, ps.Message)).
			SetTime(time.Now())
		// Flush writes
		influxdb2api.WritePoint(p)
		influxdb2api.Flush()
	}

	if ps.Status == kubeproberv1.CheckerStatusError {
		t := &ticket.Ticket{
			Kind:   ticket.ErrorTicket,
			Labels: []string{ps.CheckerName, ps.ProbeName, ps.ClusterName, "巡检"},
		}
		t.Title = fmt.Sprintf("(请勿改标题)巡检异常-[集群]: %s,[类别]: %s,[检查项]：%s",
			ps.ClusterName, ps.ProbeName, ps.CheckerName)
		t.Content = fmt.Sprintf("[集群]: %s\n[类别]: %s\n[检查项]：%s\n[检查状态]：%s\n[错误信息]：\n%s",
			ps.ClusterName, ps.ProbeName, ps.CheckerName, ps.Status, ps.Message)
		t.Type = erda_api.IssueTypeTicket
		t.Priority = erda_api.IssuePriorityHigh

		//ticket.SendTicket(t)

		if err = dingding.SendAlert(&ps); err != nil {
			errMsg := fmt.Sprintf("send dingding alert err: %+v\n", err)
			klog.Errorf(errMsg)
		}
	} else if ps.Status == kubeproberv1.CheckerStatusPass {
		t := &ticket.Ticket{Kind: ticket.PassTicket}
		t.Title = fmt.Sprintf("(请勿改标题)巡检异常-[集群]: %s,[类别]: %s,[检查项]：%s",
			ps.ClusterName, ps.ProbeName, ps.CheckerName)
		t.Content = fmt.Sprintf("[集群]: %s\n[类别]: %s\n[检查项]：%s\n[检查状态]: %s\n[错误信息]：\n%s",
			ps.ClusterName, ps.ProbeName, ps.CheckerName, ps.Status, ps.Message)
		t.Type = erda_api.IssueTypeTicket
		t.Priority = erda_api.IssuePriorityLow

		//ticket.SendTicket(t)
	}

	rw.WriteHeader(http.StatusOK)
	return
}
