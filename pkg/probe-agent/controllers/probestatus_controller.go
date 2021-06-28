// Copyright (c) 2021 Terminus, Inc.
//
// This program is free software: you can use, redistribute, and/or modify
// it under the terms of the GNU Affero General Public License, version 3
// or later ("AGPL"), as published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logger "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	probev1 "github.com/erda-project/kubeprober/pkg/probe-agent/apis/v1"
	probestatus "github.com/erda-project/kubeprober/pkg/probe-status"
)

// ProbeStatusReconciler reconciles a ProbeStatus object
type ProbeStatusReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
}

func (r *ProbeStatusReconciler) initLogger(ctx context.Context) {
	r.log = logger.FromContext(ctx)
}

//+kubebuilder:rbac:groups=kubeprober.erda.cloud,resources=probestatuses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubeprober.erda.cloud,resources=probestatuses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubeprober.erda.cloud,resources=probestatuses/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ProbeStatus object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *ProbeStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.initLogger(ctx)
	pod := corev1.Pod{}
	err := r.Get(ctx, req.NamespacedName, &pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.log.V(1).Info("could not found probe pod")
			return ctrl.Result{}, nil
		} else {
			r.log.V(1).Error(err, "could not get probe pod")
			return ctrl.Result{}, err
		}
	}

	err = probeLabelsCheck(pod.Labels)
	if err != nil {
		r.log.V(1).Error(err, "probe labels check failed", "labels", pod.Labels)
		return ctrl.Result{}, err
	}

	failed, status := FilterFailedStatus(pod.Status, pod.Labels)
	if !failed {
		return ctrl.Result{}, nil
	}

	rps := probestatus.ReportProbeStatusSpec{
		ProbeNamespace: pod.Labels[probev1.LabelKeyProbeNameSpace],
		ProbeName:      pod.Labels[probev1.LabelKeyProbeName],
		ProbeItemStatus: probev1.ProbeItemStatus{
			ProbeCheckerStatus: status,
		},
	}

	err = ReportProbeResult(r.Client, rps)
	if err != nil {
		r.log.V(1).Error(err, "report probe result failed", "content", rps)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func FilterFailedStatus(p corev1.PodStatus, labels map[string]string) (failed bool, status probev1.ProbeCheckerStatus) {
	if p.Phase == corev1.PodRunning || p.Phase == corev1.PodSucceeded {
		return
	}
	if p.Phase == corev1.PodPending && len(p.Conditions) != 0 {
		for _, c := range p.Conditions {
			if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse {
				failed = true
				pItemName := labels[probev1.LabelKeyProbeItemName]
				status = genProbeCheckerStatus(c.Reason, c.Message, pItemName)
				return
			}
		}
	}
	if p.Phase == corev1.PodFailed && p.Reason != "" {
		failed = true
		pItemName := labels[probev1.LabelKeyProbeItemName]
		status = genProbeCheckerStatus(p.Reason, p.Message, pItemName)
		return
	}
	return
}

func genProbeCheckerStatus(reason, msg, pItemName string) (status probev1.ProbeCheckerStatus) {
	now := metav1.Now()
	status = probev1.ProbeCheckerStatus{
		Name:    pItemName,
		Status:  probev1.CheckerStatusUNKNOWN,
		Message: fmt.Sprintf("pod running failed, reason:%s, message:%s", reason, msg),
		LastRun: &now,
	}
	return
}

func probeLabelsCheck(labels map[string]string) (err error) {
	if len(labels) == 0 {
		err = fmt.Errorf("empty labels")
		return
	}
	pNamespace := labels[probev1.LabelKeyProbeNameSpace]
	pName := labels[probev1.LabelKeyProbeName]
	pItemName := labels[probev1.LabelKeyProbeItemName]
	if pNamespace == "" || pName == "" || pItemName == "" {
		err = fmt.Errorf("invalid probe label info, some is empty, probeNamespace:%s, probeName:%s, pItemName:%s", pNamespace, pName, pItemName)
		return
	}
	return
}

func ReportProbeResult(c client.Client, r probestatus.ReportProbeStatusSpec) error {
	ctx := context.Background()
	ps := probev1.ProbeStatus{}
	key := client.ObjectKey{Namespace: r.ProbeNamespace, Name: r.ProbeName}
	err := c.Get(ctx, key, &ps)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ps := newProbeStatus(r)
			err := c.Create(ctx, &ps)
			if err != nil {
				logger.Log.V(1).Error(err, "create probe status failed", "content", r)
				return err
			} else {
				logger.Log.V(1).Info("create probe status successfully", "content", r)
				return nil
			}
		} else {
			logger.Log.V(1).Error(err, "get probe status failed", "content", r)
			return err
		}
	}

	needUpdate, ups := mergeProbeStatus(r, ps)
	// TODO: optimize using patch method
	if needUpdate {
		err = c.Update(ctx, &ups)
		if err != nil {
			logger.Log.V(1).Error(err, "update probe status failed", "content", r)
			return err
		}
		logger.Log.V(1).Info("update probe status successfully", "content", r)
	} else {
		logger.Log.V(1).Info("ignore duplicate status report", "content", r)
	}
	return nil
}

// probe status not exist, create it based on the incoming one probe item status
func newProbeStatus(r probestatus.ReportProbeStatusSpec) (s probev1.ProbeStatus) {
	s = probev1.ProbeStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.ProbeName,
			Namespace: r.ProbeNamespace,
		},
		Spec: probev1.ProbeStatusSpec{
			Namespace: r.ProbeNamespace,
			ProbeCheckerStatus: probev1.ProbeCheckerStatus{
				Name:    r.ProbeName,
				Status:  r.Status,
				Message: r.Message,
				LastRun: r.LastRun,
			},
			Detail: []probev1.ProbeItemStatus{r.ProbeItemStatus},
		},
	}
	return
}

func mergeProbeStatus(r probestatus.ReportProbeStatusSpec, s probev1.ProbeStatus) (bool, probev1.ProbeStatus) {

	lastRun := r.LastRun
	overwrite := true
	exist := false
	update := true

	for i, j := range s.Spec.Detail {
		if j.Name != r.Name && j.Status.Priority() > r.Status.Priority() {
			overwrite = false
		}
		if j.Name == r.Name {
			s.Spec.Detail[i] = r.ProbeItemStatus
			if !needUpdate(r.ProbeCheckerStatus, j.ProbeCheckerStatus) {
				update = false
			}
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
	return update, s
}

// needUpdate: prevent frequently update
func needUpdate(new, old probev1.ProbeCheckerStatus) bool {
	// TODO: check interval could change depend on runInterval
	if new.Status == old.Status && new.Message == old.Message && (new.LastRun.Sub(old.LastRun.Time) < 2*time.Minute) {
		return false
	}
	return true
}

// filter exception pod
type PodPredicates struct {
	predicate.Funcs
}

func (p *PodPredicates) Create(e event.CreateEvent) bool {
	// TODO: when controller start, create event generated for exist crd resource; should ignore finished probe job, maybe status field needed
	n := getNamespaceName(e.Object)
	logger.Log.V(2).Info("ignore create event for probe pod", "pod", n)
	return false
}

func (p *PodPredicates) Delete(e event.DeleteEvent) bool {
	n := getNamespaceName(e.Object)
	logger.Log.V(2).Info("ignore delete event for probe pod", "pod", n)
	return false
}

func (p *PodPredicates) Update(e event.UpdateEvent) bool {

	oldObject := e.ObjectOld.(*corev1.Pod)
	newObject := e.ObjectNew.(*corev1.Pod)

	if newObject.Status.Phase != corev1.PodPending && newObject.Status.Phase != corev1.PodFailed {
		return false
	}

	if oldObject.Status.Phase == newObject.Status.Phase &&
		oldObject.Status.Reason == newObject.Status.Reason {
		return false
	}

	n := getNamespaceName(e.ObjectNew)
	logger.Log.V(2).Info("update event for probe pod", "pod", n)
	return true
}

func (p *PodPredicates) Generic(e event.GenericEvent) bool {
	n := getNamespaceName(e.Object)
	logger.Log.V(2).Info("generic event for probe pod", "pod", n)
	return true
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProbeStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	podPredicates := builder.WithPredicates(&PodPredicates{})
	return ctrl.NewControllerManagedBy(mgr).
		// watch pod, get failed probe pod and update related probe status
		For(&corev1.Pod{}, podPredicates).
		// watch probe status but do nothing, only cache & sync probe status
		Watches(&source.Kind{Type: &probev1.ProbeStatus{}}, handler.Funcs{}).
		Complete(r)
}
