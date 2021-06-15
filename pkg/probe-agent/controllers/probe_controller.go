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

	probev1alpha1 "github.com/erda-project/kubeprober/pkg/probe-agent/apis/v1alpha1"
	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
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

// ProbeReconciler reconciles a Probe object
type ProbeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
}

func (r *ProbeReconciler) initLogger(ctx context.Context) {
	r.log = logger.FromContext(ctx)
}

//+kubebuilder:rbac:groups=kubeprober.erda.cloud,resources=probes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubeprober.erda.cloud,resources=probes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubeprober.erda.cloud,resources=probes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Probe object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *ProbeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.initLogger(ctx)
	r.log.V(1).Info("reconcile probe task")

	// check whether probe been deleted
	var probe probev1alpha1.Probe
	err := r.Get(ctx, req.NamespacedName, &probe)
	if err != nil {
		// probe deleted, ignore
		if apierrors.IsNotFound(err) {
			r.log.V(1).Info("could not found probe task")
			return ctrl.Result{}, nil
		} else {
			r.log.V(1).Error(err, "could not get probe task")
			return ctrl.Result{}, err
		}
	}
	// check whether it's single probe or cron probe
	if probe.Spec.Policy.RunInterval <= 0 {
		return r.ReconcileJobs(ctx, &probe)
	} else {
		return r.ReconcileCronJobs(ctx, req, &probe)
	}
}

func (r *ProbeReconciler) ReconcileJobs(ctx context.Context, probe *probev1alpha1.Probe) (ctrl.Result, error) {

	r.log.V(0).Info("reconcile probe jobs")

	for _, j := range probe.Spec.ProbeList {
		_, err := r.ReconcileJob(ctx, j, probe)
		if err != nil {
			r.log.V(1).Error(err, "reconcile job failed")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *ProbeReconciler) ReconcileJob(ctx context.Context, pItem probev1alpha1.ProbeItem, probe *probev1alpha1.Probe) (ctrl.Result, error) {
	n := client.ObjectKey{Namespace: probe.Namespace, Name: pItem.Name}
	r.log.V(0).Info("reconcile probe job", "job", n)

	// check whether probe been deleted
	var job batchv1.Job
	err := r.Get(ctx, n, &job)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// job crate, if not found
			r.log.V(1).Info("could not found probe job, create it", "job", n)
			if err := r.createJob(ctx, pItem, probe); err != nil {
				r.log.V(1).Error(err, "create probe job failed")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		} else {
			r.log.V(1).Error(err, "could not get probe job", "job", n)
			return ctrl.Result{}, err
		}
	}

	// job update, not support
	r.log.V(1).Info("probe job already exist", "job", n)

	return ctrl.Result{}, nil
}

func (r *ProbeReconciler) createJob(ctx context.Context, pItem probev1alpha1.ProbeItem, probe *probev1alpha1.Probe) error {
	j, err := genJob(pItem, probe)
	if err != nil {
		return err
	}
	err = r.Create(ctx, &j)
	if err != nil {
		return err
	}
	return nil
}

func genJob(pItem probev1alpha1.ProbeItem, probe *probev1alpha1.Probe) (j batchv1.Job, err error) {
	if pItem.Name == "" {
		err = fmt.Errorf("prob item with empty name is not allowed")
		return
	}

	// TODO: add annotations & labels to mark parent crd resource; add random postfix to job name
	trueVar := true
	j = batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pItem.Name,
			Namespace: probe.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: probe.APIVersion,
					Kind:       probe.Kind,
					Name:       probe.Name,
					UID:        probe.UID,
					Controller: &trueVar,
				},
			},
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: pItem.Spec,
			},
		},
		Status: batchv1.JobStatus{},
	}

	return
}

func (r *ProbeReconciler) ReconcileCronJobs(ctx context.Context, req ctrl.Request, probe *probev1alpha1.Probe) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

type ProbeEventPredicates struct {
	predicate.Funcs
}

func (p *ProbeEventPredicates) Create(e event.CreateEvent) bool {
	// TODO: when controller start, create event generated for exist crd resource; should ignore finished probe job, maybe status field needed
	n := fmt.Sprintf("%s/%s", e.Object.GetNamespace(), e.Object.GetName())
	logger.Log.V(1).Info("create event for probe task", "task", n)
	return true
}

func (p *ProbeEventPredicates) Delete(e event.DeleteEvent) bool {
	n := fmt.Sprintf("%s/%s", e.Object.GetNamespace(), e.Object.GetName())
	logger.Log.V(1).Info("ignore delete event for probe task", "task", n)
	return false
}

func (p *ProbeEventPredicates) Update(e event.UpdateEvent) bool {
	n := fmt.Sprintf("%s/%s", e.ObjectNew.GetNamespace(), e.ObjectNew.GetName())
	logger.Log.V(1).Info("ignore update event for probe task", "task", n)
	return false
}

func (p *ProbeEventPredicates) Generic(e event.GenericEvent) bool {
	n := fmt.Sprintf("%s/%s", e.Object.GetNamespace(), e.Object.GetName())
	logger.Log.V(1).Info("generic event for probe task", "task", n)
	return true
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProbeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	probeEventPredicates := builder.WithPredicates(&ProbeEventPredicates{})

	return ctrl.NewControllerManagedBy(mgr).
		For(&probev1alpha1.Probe{}, probeEventPredicates).
		Complete(r)
}
