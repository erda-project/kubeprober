package webserver

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	probev1alpha1 "github.com/erda-project/kubeprober/pkg/probe-agent/apis/v1alpha1"
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

	err = s.ProbeResultHandlerInternal(rp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logrus.Errorf("process probe item status failed, probe item status:%v, error:%v", rp, err)
		return nil
	}

	w.WriteHeader(http.StatusOK)
	logrus.Infof("process probe item status successfully, key: %s/%s/%s", rp.ProbeNamespace, rp.ProbeName, rp.Name)
	return nil
}

func (s *Server) ProbeResultHandlerInternal(r probestatus.ReportProbeStatusSpec) error {
	ctx := context.Background()
	ps := probev1alpha1.ProbeStatus{}
	key := client.ObjectKey{Namespace: r.ProbeNamespace, Name: r.ProbeName}
	err := s.Client().Get(ctx, key, &ps)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ps := newProbeStatus(r)
			err := s.Client().Create(ctx, &ps)
			if err != nil {
				logrus.Errorf("create probe status failed, content: %v, error:%v", r, err)
				return err
			} else {
				logrus.Infof("create probe status successfully, key: %v", key)
				return nil
			}
		} else {
			logrus.Errorf("get probe status failed, content: %v, error:%v", r, err)
			return err
		}
	}

	ups := updateProbeStatus(r, ps)
	// TODO: optimize using patch method
	err = s.Client().Update(ctx, &ups)
	if err != nil {
		logrus.Errorf("patch probe status failed, current:%+v, patch:%+v, error:%v", ps, r, err)
		return err
	}
	return nil
}

// probe status not exist, create it based on the incoming one probe item status
func newProbeStatus(r probestatus.ReportProbeStatusSpec) (s probev1alpha1.ProbeStatus) {
	s = probev1alpha1.ProbeStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.ProbeName,
			Namespace: r.ProbeNamespace,
		},
		Spec: probev1alpha1.ProbeStatusSpec{
			Namespace: r.ProbeNamespace,
			ProbeCheckerStatus: probev1alpha1.ProbeCheckerStatus{
				Name:    r.ProbeName,
				Status:  r.Status,
				Message: r.Message,
				LastRun: r.LastRun,
			},
			Detail: []probev1alpha1.ProbeItemStatus{r.ProbeItemStatus},
		},
	}
	return
}

func updateProbeStatus(r probestatus.ReportProbeStatusSpec, s probev1alpha1.ProbeStatus) probev1alpha1.ProbeStatus {

	lastRun := r.LastRun
	overwrite := true
	exist := false

	for i, j := range s.Spec.Detail {
		if j.Name != r.Name && j.Status.Priority() > r.Status.Priority() {
			overwrite = false
		}
		if j.Name == r.Name {
			s.Spec.Detail[i] = r.ProbeItemStatus
			exist = true
		}
		if j.LastRun.After(lastRun.Time) {
			lastRun = j.LastRun
		}
	}

	if !exist {
		s.Spec.Detail = append(s.Spec.Detail, r.ProbeItemStatus)
	}

	s.Spec.LastRun = lastRun
	if overwrite {
		s.Spec.Status = r.Status
		s.Spec.Message = r.Message
	}
	logrus.Infof("report status:%+v, update status:%+v", r, s)
	return s
}
