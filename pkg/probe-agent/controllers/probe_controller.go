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
	"crypto/md5"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logger "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/erda-project/kubeprober/cmd/probe-agent/options"
	probev1 "github.com/erda-project/kubeprober/pkg/probe-agent/apis/v1"
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
	var probe probev1.Probe
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

	//update probe status
	probeSpecByte, _ := json.Marshal(probe.Spec)
	probeSpecHas := fmt.Sprintf("%x", md5.Sum(probeSpecByte))
	if probe.Status.MD5 != fmt.Sprintf("%x", probeSpecHas) {
		probe.Status.MD5 = probeSpecHas
		err := r.Status().Update(ctx, &probe)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// check whether it's single probe or cron probe
	if probe.Spec.Policy.RunInterval <= 0 {
		return r.ReconcileJobs(ctx, &probe)
	} else {
		return r.ReconcileCronJobs(ctx, &probe)
	}
}

func (r *ProbeReconciler) ReconcileJobs(ctx context.Context, probe *probev1.Probe) (ctrl.Result, error) {
	for _, j := range probe.Spec.ProbeList {
		_, err := r.ReconcileJob(ctx, j, probe)
		if err != nil {
			r.log.V(1).Error(err, "reconcile job failed")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *ProbeReconciler) ReconcileJob(ctx context.Context, pItem probev1.ProbeItem, probe *probev1.Probe) (ctrl.Result, error) {
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

func (r *ProbeReconciler) ReconcileCronJobs(ctx context.Context, probe *probev1.Probe) (ctrl.Result, error) {

	r.log.V(0).Info("reconcile probe cron jobs")

	for _, j := range probe.Spec.ProbeList {
		_, err := r.ReconcileCronJob(ctx, j, probe)
		if err != nil {
			r.log.V(1).Error(err, "reconcile cron job failed")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *ProbeReconciler) ReconcileCronJob(ctx context.Context, pItem probev1.ProbeItem, probe *probev1.Probe) (ctrl.Result, error) {
	n := client.ObjectKey{Namespace: probe.Namespace, Name: pItem.Name}
	r.log.V(0).Info("reconcile probe cron job", "cronjob", n)

	// check whether probe been deleted
	var cj batchv1beta1.CronJob
	err := r.Get(ctx, n, &cj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// job crate, if not found
			r.log.V(1).Info("could not found probe cron job, create it", "cronjob", n)
			if err := r.CreateCronJob(ctx, pItem, probe); err != nil {
				r.log.V(1).Error(err, "create probe cron job failed", "cronjob", n)
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		} else {
			r.log.V(1).Error(err, "could not get probe cron job", "cronjob", n)
			return ctrl.Result{}, err
		}
	}

	// cron job update
	r.log.V(0).Info("update probe cron job", "cronjob", n)
	err = r.UpdateCronJob(ctx, pItem, probe)
	if err != nil {
		r.log.V(0).Error(err, "update probe cron job failed", "cronjob", n)
	}
	return ctrl.Result{}, nil
}

func (r *ProbeReconciler) CreateCronJob(ctx context.Context, pItem probev1.ProbeItem, probe *probev1.Probe) error {
	cj, err := genCronJob(pItem, probe)
	if err != nil {
		return err
	}
	err = r.Create(ctx, &cj)
	if err != nil {
		return err
	}
	return nil
}

func (r *ProbeReconciler) UpdateCronJob(ctx context.Context, pItem probev1.ProbeItem, probe *probev1.Probe) error {
	cj, err := genCronJob(pItem, probe)
	if err != nil {
		return err
	}
	err = r.Update(ctx, &cj)
	if err != nil {
		return err
	}
	return nil
}

func (r *ProbeReconciler) createJob(ctx context.Context, pItem probev1.ProbeItem, probe *probev1.Probe) error {
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

func genCronJob(pItem probev1.ProbeItem, probe *probev1.Probe) (cj batchv1beta1.CronJob, err error) {
	j, err := genJob(pItem, probe)
	if err != nil {
		return
	}
	trueVar := true
	schedule := fmt.Sprintf("*/%d * * * *", probe.Spec.Policy.RunInterval)
	cj = batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: probe.Namespace,
			Name:      pItem.Name,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: probe.APIVersion,
					Kind:       probe.Kind,
					Name:       probe.Name,
					UID:        probe.UID,
					Controller: &trueVar,
				},
			},
			Labels: map[string]string{
				probev1.LabelKeyApp:            probev1.LabelValueApp,
				probev1.LabelKeyProbeNameSpace: probe.Namespace,
				probev1.LabelKeyProbeName:      probe.Name,
				probev1.LabelKeyProbeItemName:  pItem.Name,
			},
		},
		Spec: batchv1beta1.CronJobSpec{
			Schedule:                schedule,
			StartingDeadlineSeconds: nil,
			ConcurrencyPolicy:       "",
			Suspend:                 nil,
			JobTemplate: batchv1beta1.JobTemplateSpec{
				Spec: j.Spec,
			},
			// TODO: gc configuration
			SuccessfulJobsHistoryLimit: nil,
			FailedJobsHistoryLimit:     nil,
		},
		Status: batchv1beta1.CronJobStatus{},
	}
	return
}

func genJob(pItem probev1.ProbeItem, probe *probev1.Probe) (j batchv1.Job, err error) {
	if pItem.Name == "" {
		err = fmt.Errorf("prob item with empty name is not allowed")
		return
	}
	envInject(&pItem, probe)
	trueVar := true
	// TODO: put this config in specific area
	activeDeadlineSecond := int64(60 * 30)
	backoffLimit := int32(0)
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
			Labels: map[string]string{
				probev1.LabelKeyApp:            probev1.LabelValueApp,
				probev1.LabelKeyProbeNameSpace: probe.Namespace,
				probev1.LabelKeyProbeName:      probe.Name,
				probev1.LabelKeyProbeItemName:  pItem.Name,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						probev1.LabelKeyApp:            probev1.LabelValueApp,
						probev1.LabelKeyProbeNameSpace: probe.Namespace,
						probev1.LabelKeyProbeName:      probe.Name,
						probev1.LabelKeyProbeItemName:  pItem.Name,
					},
				},
				Spec: pItem.Spec,
			},
			ActiveDeadlineSeconds: &activeDeadlineSecond,
			BackoffLimit:          &backoffLimit,
		},
	}

	return
}

func envInject(pItem *probev1.ProbeItem, probe *probev1.Probe) {
	set := map[string]string{
		probev1.ProbeNamespace: "",
		probev1.ProbeName:      "",
		probev1.ProbeItemName:  "",
	}
	ienvs := []corev1.EnvVar{
		{
			Name:  probev1.ProbeNamespace,
			Value: probe.Namespace,
		},
		{
			Name:  probev1.ProbeName,
			Value: probe.Name,
		},
		{
			Name:  probev1.ProbeItemName,
			Value: pItem.Name,
		},
		{
			Name:  probev1.ProbeStatusReportUrl,
			Value: options.ProbeAgentConf.GetProbeStatusReportUrl(),
		},
	}
	for i := range pItem.Spec.Containers {
		env := pItem.Spec.Containers[i].Env
		for j, e := range env {
			if _, ok := set[e.Name]; ok {
				env = remove(env, j)
			}
		}
		env = append(env, ienvs...)
		pItem.Spec.Containers[i].Env = env
	}
}

func remove(slice []corev1.EnvVar, s int) []corev1.EnvVar {
	return append(slice[:s], slice[s+1:]...)
}

type ProbePredicates struct {
	predicate.Funcs
}

func (p *ProbePredicates) Create(e event.CreateEvent) bool {
	return true
}

func (p *ProbePredicates) Delete(e event.DeleteEvent) bool {
	return false
}

func (p *ProbePredicates) Update(e event.UpdateEvent) bool {
	oldObject := e.ObjectOld.(*probev1.Probe)
	newObject := e.ObjectNew.(*probev1.Probe)
	equal := cmp.Equal(oldObject.Spec, newObject.Spec)
	if !equal {
		return true
	}
	return false
}

func (p *ProbePredicates) Generic(e event.GenericEvent) bool {
	return true
}

type ProbeCronJobPredicates struct {
	predicate.Funcs
}

func (pcj *ProbeCronJobPredicates) Create(e event.CreateEvent) bool {
	return false
}

func (pcj *ProbeCronJobPredicates) Delete(e event.DeleteEvent) bool {
	return true
}

func (pcj *ProbeCronJobPredicates) Update(e event.UpdateEvent) bool {
	oldObject := e.ObjectOld.(*batchv1beta1.CronJob)
	newObject := e.ObjectNew.(*batchv1beta1.CronJob)
	equal := cmp.Equal(oldObject.Spec, newObject.Spec)
	if !equal {
		return true
	}
	return false
}

func (pcj *ProbeCronJobPredicates) Generic(e event.GenericEvent) bool {
	return true
}

func getNamespaceName(o client.Object) string {
	return fmt.Sprintf("%s/%s", o.GetNamespace(), o.GetName())
}

func getProbeNamespaceName(o client.Object) string {
	labels := o.GetLabels()
	return fmt.Sprintf("%s/%s", labels[probev1.LabelKeyProbeNameSpace], labels[probev1.LabelKeyProbeName])
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProbeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	probePredicates := builder.WithPredicates(&ProbePredicates{})
	probeCronJobPredicates := builder.WithPredicates(&ProbeCronJobPredicates{})

	return ctrl.NewControllerManagedBy(mgr).
		For(&probev1.Probe{}, probePredicates).
		Owns(&batchv1beta1.CronJob{}, probeCronJobPredicates).
		Complete(r)
}
