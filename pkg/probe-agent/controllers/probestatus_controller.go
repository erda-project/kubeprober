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

package controllers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kubeprobev1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logger "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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
	var patch []byte
	var err error

	//update status of probestatus
	ps := kubeprobev1.ProbeStatus{}
	if err = r.Get(ctx, req.NamespacedName, &ps); err != nil {
		if apierrors.IsNotFound(err) {
			//r.log.V(1).Info("could not found probestatus")
			return ctrl.Result{}, nil
		} else {
			r.log.V(1).Error(err, "could not get probestatus")
			return ctrl.Result{}, err
		}
	}

	var fStatus kubeprobev1.CheckerStatus
	var fMessage string
	var fLastRun *metav1.Time
	for _, i := range ps.Spec.Checkers {
		if fStatus == "" {
			fStatus = i.Status
			fMessage = i.Message
		}
		if i.Status.Priority() > fStatus.Priority() {
			fStatus = i.Status
			fMessage = i.Message
		}
		if fLastRun == nil {
			fLastRun = i.LastRun
		}
		if fLastRun.Before(i.LastRun) {
			fLastRun = i.LastRun
		}
	}
	if fMessage == "" {
		fMessage = "-"
	}
	statusPatch := kubeprobev1.ProbeStatus{
		Status: kubeprobev1.ProbeStatusStates{
			Message: fMessage,
			Status:  fStatus,
			LastRun: fLastRun,
		},
	}
	if patch, err = json.Marshal(statusPatch); err != nil {
		return ctrl.Result{}, err
	}
	if err = r.Status().Patch(ctx, &kubeprobev1.ProbeStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ps.Name,
			Namespace: ps.Namespace,
		},
	}, client.RawPatch(types.MergePatchType, patch)); err != nil {
		r.log.V(1).Error(err, "update probestatus status error", "probestatus", ps.Name)
		return ctrl.Result{}, err
	}

	pod := corev1.Pod{}
	err = r.Get(ctx, req.NamespacedName, &pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			//r.log.V(1).Info("could not found probe pod")
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

	rps := kubeproberv1.ReportProbeStatusSpec{
		ProbeNamespace:     pod.Labels[kubeprobev1.LabelKeyProbeNameSpace],
		ProbeName:          pod.Labels[kubeprobev1.LabelKeyProbeName],
		ProbeCheckerStatus: status,
	}

	err = ReportProbeResult(r.Client, rps)
	if err != nil {
		r.log.V(1).Error(err, "report probe result failed", "content", rps)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func FilterFailedStatus(p corev1.PodStatus, labels map[string]string) (failed bool, status kubeprobev1.ProbeCheckerStatus) {
	if p.Phase == corev1.PodRunning || p.Phase == corev1.PodSucceeded {
		return
	}
	if p.Phase == corev1.PodPending && len(p.Conditions) != 0 {
		for _, c := range p.Conditions {
			if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse {
				failed = true
				pName := labels[kubeprobev1.LabelKeyProbeName]
				status = genProbeCheckerStatus(c.Reason, c.Message, pName)
				return
			}
		}
	}
	if p.Phase == corev1.PodFailed && p.Reason != "" {
		failed = true
		pName := labels[kubeprobev1.LabelKeyProbeName]
		status = genProbeCheckerStatus(p.Reason, p.Message, pName)
		return
	}
	return
}

func genProbeCheckerStatus(reason, msg, pName string) (status kubeprobev1.ProbeCheckerStatus) {
	now := metav1.Now()
	status = kubeprobev1.ProbeCheckerStatus{
		Name:    pName,
		Status:  kubeprobev1.CheckerStatusUNKNOWN,
		Message: fmt.Sprintf("pod running failed, reason: %s, message: %s", reason, msg),
		LastRun: &now,
	}
	return
}

func probeLabelsCheck(labels map[string]string) (err error) {
	if len(labels) == 0 {
		err = fmt.Errorf("empty labels")
		return
	}
	pNamespace := labels[kubeprobev1.LabelKeyProbeNameSpace]
	pName := labels[kubeprobev1.LabelKeyProbeName]
	if pNamespace == "" || pName == "" {
		err = fmt.Errorf("invalid probe label info, some is empty, probeNamespace:%s, probeName:%s", pNamespace, pName)
		return
	}
	return
}

func ReportProbeResult(c client.Client, r kubeproberv1.ReportProbeStatusSpec) error {
	ctx := context.Background()
	ps := kubeprobev1.ProbeStatus{}
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
	logger.Log.V(2).Info("status merge info", "incoming probe status", r, "need update", needUpdate, "before merge", ps, "after merge", ups)
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
func newProbeStatus(r kubeproberv1.ReportProbeStatusSpec) (s kubeprobev1.ProbeStatus) {
	s = kubeprobev1.ProbeStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.ProbeName,
			Namespace: r.ProbeNamespace,
		},
		Spec: kubeprobev1.ProbeStatusSpec{
			Checkers: r.Checkers,
		},
	}
	return
}

func mergeProbeStatus(r kubeproberv1.ReportProbeStatusSpec, s kubeprobev1.ProbeStatus) (bool, kubeprobev1.ProbeStatus) {

	var existCheckerList []string
	// check whether need to update probe status
	update := true
	for _, i := range s.Spec.Checkers {
		existCheckerList = append(existCheckerList, i.Name)
	}
	for _, i := range r.Checkers {
		index, flag := IsContain(existCheckerList, i.Name)
		if flag {
			s.Spec.Checkers[index].Status = i.Status
			s.Spec.Checkers[index].Message = i.Message
			s.Spec.Checkers[index].LastRun = i.LastRun
		} else {
			s.Spec.Checkers = append(s.Spec.Checkers, i)
		}
	}
	return update, s
}

// needUpdate: prevent frequently update
func needUpdate(new, old kubeprobev1.ProbeCheckerStatus) bool {
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

// filter probestatus
type ProbeStatusPredicates struct {
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
	n := getNamespaceName(e.ObjectNew)
	oldObject := e.ObjectOld.(*corev1.Pod)
	newObject := e.ObjectNew.(*corev1.Pod)

	if newObject.Status.Phase != corev1.PodPending && newObject.Status.Phase != corev1.PodFailed {
		return false
	}

	if cmp.Equal(oldObject.Status, newObject.Status) {
		return false
	}
	logger.Log.V(2).Info("update event for probe pod, status change", "pod", n, "status new", newObject.Status, "status old", oldObject.Status)
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
		//watch pod, get failed probe pod and update related probe status
		For(&corev1.Pod{}, podPredicates).
		// watch probe status but do nothing, only cache & sync probe status
		Watches(&source.Kind{Type: &kubeprobev1.ProbeStatus{}}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

func IsContain(items []string, item string) (int, bool) {
	for index, eachItem := range items {
		if eachItem == item {
			return index, true
		}
	}
	return 0, false
}
