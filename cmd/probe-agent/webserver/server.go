package webserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/erda-project/kubeprober/pkg/probe-agent/controllers"
	probestatus "github.com/erda-project/kubeprober/pkg/probe-status"
)

type Server struct {
	client          client.Client
	ProbeListenAddr string // the listen address, such as ":80"
}

func NewServer(c client.Client, addr string) Server {
	s := Server{client: c, ProbeListenAddr: addr}
	return s
}

func (s *Server) Start() {
	go func() {
		// Accept status reports coming from external checker pods
		http.HandleFunc("/probe-status", func(w http.ResponseWriter, r *http.Request) {
			err := s.ProbeResultHandler(w, r)
			if err != nil {
				logrus.Errorf("probe-status endpoint error: %v", err)
			}
		})

		for {
			logrus.Infof("starting web server on port: %s", s.ProbeListenAddr)
			err := http.ListenAndServe(s.ProbeListenAddr, nil)
			if err != nil {
				logrus.Errorf("start web server failed, port:%s, error:%v", s.ProbeListenAddr, err)
				time.Sleep(time.Second)
			}
		}

	}()
}

func (s Server) Client() client.Client {
	return s.client
}

func (s *Server) ProbeResultHandler(w http.ResponseWriter, r *http.Request) error {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		logrus.Errorf("read request body failed, body: %s, error:%v", string(b), err)
		return nil
	}

	rp := probestatus.ReportProbeStatusSpec{}
	err = json.Unmarshal(b, &rp)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		logrus.Errorf("unmarshal request body failed, body: %s, error:%v", string(b), err)
		return nil
	}

	err = controllers.ReportProbeResult(s.client, rp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logrus.Errorf("process probe item status failed, probe item status:%v, error:%v", rp, err)
		return nil
	}

	w.WriteHeader(http.StatusOK)
	logrus.Infof("process probe item status successfully, key: %s/%s/%s", rp.ProbeNamespace, rp.ProbeName, rp.Name)
	return nil
}
