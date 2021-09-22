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
	"crypto/md5"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logger "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/cmd/probe-agent/options"
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
//+kubebuilder:rbac:groups="",resources=nodes,verbs=list;watch;get
//+kubebuilder:rbac:groups="",resources=pods,verbs=list;watch;get
//+kubebuilder:rbac:groups="*",resources="*",verbs="*"
//+kubebuilder:rbac:groups="batch",resources=jobs,verbs=create;get;list;watch;delete;update;patch;deletecollection
//+kubebuilder:rbac:groups="batch",resources=cronjobs,verbs=create;get;list;watch;delete;update;patch;deletecollection

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
	var probe kubeproberv1.Probe
	var patch []byte
	var err error

	err = r.Get(ctx, req.NamespacedName, &probe)
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

	if probe.Status.Phase == kubeproberv1.OnceProbeDonePhase {
		return ctrl.Result{}, nil
	}
	//update probe status
	// check whether it's single probe or cron probe
	var phase string
	if probe.Spec.Policy.RunInterval <= 0 {
		phase = kubeproberv1.OnceProbeDonePhase
	} else {
		phase = ""
	}
	probeSpecByte, _ := json.Marshal(probe.Spec)
	probeSpecHas := fmt.Sprintf("%x", md5.Sum(probeSpecByte))
	if probe.Status.MD5 != fmt.Sprintf("%x", probeSpecHas) {
		//update status of cluster
		statusPatch := kubeproberv1.Probe{
			Status: kubeproberv1.ProbeStates{
				MD5:   probeSpecHas,
				Phase: phase,
			},
		}
		if patch, err = json.Marshal(statusPatch); err != nil {
			return ctrl.Result{}, err
		}
		if err = r.Status().Patch(ctx, &kubeproberv1.Probe{
			ObjectMeta: metav1.ObjectMeta{
				Name:      probe.Name,
				Namespace: probe.Namespace,
			},
		}, client.RawPatch(types.MergePatchType, patch)); err != nil {
			r.log.V(1).Error(err, "update cluster status error")
			if !strings.Contains(err.Error(), "could not find the requested resource") {
				return ctrl.Result{}, err
			}
		}

	}

	// check whether it's single probe or cron probe
	if probe.Spec.Policy.RunInterval <= 0 {
		return r.ReconcileJobs(ctx, &probe)
	} else {
		return r.ReconcileCronJobs(ctx, &probe)
	}
}

func (r *ProbeReconciler) ReconcileJobs(ctx context.Context, probe *kubeproberv1.Probe) (ctrl.Result, error) {
	_, err := r.ReconcileJob(ctx, probe)
	if err != nil {
		r.log.V(1).Error(err, "reconcile job failed")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ProbeReconciler) ReconcileJob(ctx context.Context, probe *kubeproberv1.Probe) (ctrl.Result, error) {
	n := client.ObjectKey{Namespace: probe.Namespace, Name: probe.Name}
	r.log.V(0).Info("reconcile probe job", "job", n)

	// check whether probe been deleted
	var job batchv1.Job
	err := r.Get(ctx, n, &job)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// job crate, if not found
			r.log.V(1).Info("could not found probe job, create it", "job", n)
			if err := r.createJob(ctx, probe); err != nil {
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

func (r *ProbeReconciler) ReconcileCronJobs(ctx context.Context, probe *kubeproberv1.Probe) (ctrl.Result, error) {
	_, err := r.ReconcileCronJob(ctx, probe)
	if err != nil {
		r.log.V(1).Error(err, "reconcile cron job failed")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ProbeReconciler) ReconcileCronJob(ctx context.Context, probe *kubeproberv1.Probe) (ctrl.Result, error) {
	n := client.ObjectKey{Namespace: probe.Namespace, Name: probe.Name}
	r.log.V(0).Info("reconcile probe cron job", "cronjob", n)

	// check whether probe been deleted
	var cj batchv1beta1.CronJob
	err := r.Get(ctx, n, &cj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// job crate, if not found
			r.log.V(1).Info("could not found probe cron job, create it", "cronjob", n)
			if err := r.CreateCronJob(ctx, probe); err != nil {
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
	err = r.UpdateCronJob(ctx, probe)
	if err != nil {
		r.log.V(0).Error(err, "update probe cron job failed", "cronjob", n)
	}
	return ctrl.Result{}, nil
}

func (r *ProbeReconciler) CreateCronJob(ctx context.Context, probe *kubeproberv1.Probe) error {
	cj, err := r.genCronJob(probe)
	if err != nil {
		return err
	}
	err = r.Create(ctx, &cj)
	if err != nil {
		return err
	}
	return nil
}

func (r *ProbeReconciler) UpdateCronJob(ctx context.Context, probe *kubeproberv1.Probe) error {
	cj, err := r.genCronJob(probe)
	if err != nil {
		return err
	}
	err = r.Update(ctx, &cj)
	if err != nil {
		return err
	}
	return nil
}

func (r *ProbeReconciler) createJob(ctx context.Context, probe *kubeproberv1.Probe) error {
	j, err := genJob(probe)
	if err != nil {
		return err
	}
	err = r.Create(ctx, &j)
	if err != nil {
		return err
	}
	return nil
}

func (r *ProbeReconciler) genCronJob(probe *kubeproberv1.Probe) (cj batchv1beta1.CronJob, err error) {
	j, err := genJob(probe)
	if err != nil {
		return
	}
	// generate random run interval if set
	randomRunInterval := r.getRunIntervalRandom(probe)

	trueVar := true
	schedule := fmt.Sprintf("*/%d * * * *", randomRunInterval)
	cj = batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: probe.Namespace,
			Name:      probe.Name,
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
				kubeproberv1.LabelKeyApp:            kubeproberv1.LabelValueApp,
				kubeproberv1.LabelKeyProbeNameSpace: probe.Namespace,
				kubeproberv1.LabelKeyProbeName:      probe.Name,
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

func (r *ProbeReconciler) getRunIntervalRandom(probe *kubeproberv1.Probe) int {
	policy := probe.Spec.Policy
	name := probe.Name
	// random not set, just return
	if policy.RunIntervalRandom <= 0 {
		return policy.RunInterval
	}

	random := policy.RunIntervalRandom
	if random > policy.RunInterval {
		random = policy.RunInterval / 3
		e := fmt.Errorf("runIntervalRandom [%v] bigger then runInterval [%v]", policy.RunIntervalRandom, policy.RunInterval)
		r.log.V(0).Error(e, "invalid runIntervalRandom")
	}

	now := time.Now().Unix()
	rand.Seed(now)
	ran := rand.Intn(random)
	if now%2 == 0 {
		ran = 0 - ran
	}

	r.log.V(0).Info("generate run interval", "probe name", name,
		"origin policy", policy, "run interval random", ran, "new run interval", ran+policy.RunInterval)

	return ran + policy.RunInterval
}

func genJob(probe *kubeproberv1.Probe) (j batchv1.Job, err error) {
	env, from := envInject(*probe)
	// TODO: put this config in specific area
	serviceAccountName := "kubeprober-worker"
	trueVar := true
	activeDeadlineSecond := int64(60 * 30)
	backoffLimit := int32(0)
	//jobTTL := int32(100)

	// default restart policy for job: "Never"
	restartPolicy := probe.Spec.Template.RestartPolicy
	if restartPolicy == "" || restartPolicy == corev1.RestartPolicyAlways {
		probe.Spec.Template.RestartPolicy = corev1.RestartPolicyNever
	}
	// default image pull policy: "Always"
	imagePullPolicy := corev1.PullAlways

	j = *Job(probe.Name,
		// job render
		JobNamespace(probe.Namespace),
		JobOwnerReference([]metav1.OwnerReference{
			{
				APIVersion: probe.APIVersion,
				Kind:       probe.Kind,
				Name:       probe.Name,
				UID:        probe.UID,
				Controller: &trueVar,
			},
		}),
		JobLabels(map[string]string{
			kubeproberv1.LabelKeyApp:            kubeproberv1.LabelValueApp,
			kubeproberv1.LabelKeyProbeNameSpace: probe.Namespace,
			kubeproberv1.LabelKeyProbeName:      probe.Name,
		}),
		// job spec render
		JobSpec(
			JobSpecTmpLabels(map[string]string{
				kubeproberv1.LabelKeyApp:            kubeproberv1.LabelValueApp,
				kubeproberv1.LabelKeyProbeNameSpace: probe.Namespace,
				kubeproberv1.LabelKeyProbeName:      probe.Name,
			}),
			JobSpecActiveDeadlineSeconds(activeDeadlineSecond),
			JobSpecBackoffLimit(backoffLimit),
			JobSpecTmpPod(probe.Spec.Template),
			JobSpecTmpServiceAccount(serviceAccountName),
			JobSpecTmpRestartPolicy(restartPolicy),
			JobSpecTmpImagePullPolicy(imagePullPolicy),
			JobSpecTmpPodEnvs(env),
			JobSpecTmpPodEnvSources(from),
		),
	)

	return
}

func envInject(probe kubeproberv1.Probe) (envs []corev1.EnvVar, envFromSources []corev1.EnvFromSource) {
	envs = make([]corev1.EnvVar, 0)
	envFromSources = make([]corev1.EnvFromSource, 0)

	// env from
	envFromSources = append(envFromSources,
		corev1.EnvFromSource{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: kubeproberv1.ExtraCMName,
				}}},
	)

	// envs
	envs = []corev1.EnvVar{
		{
			Name:  kubeproberv1.ProbeNamespace,
			Value: probe.Namespace,
		},
		{
			Name:  kubeproberv1.ProbeName,
			Value: probe.Name,
		},
		{
			Name:  kubeproberv1.ProbeStatusReportUrl,
			Value: options.ProbeAgentConf.GetProbeStatusReportUrl(),
		},
	}

	// env from probe configs
	for i := range probe.Spec.Configs {
		for j := range probe.Spec.Configs[i].Env {
			envs = append(envs, probe.Spec.Configs[i].Env[j])
		}
	}

	return
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
	ns := getNamespaceName(e.ObjectNew)
	logger.Log.V(2).Info("probe update", "key", ns)
	if e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration() {
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
	ns := getNamespaceName(e.ObjectNew)
	logger.Log.V(2).Info("cronjob update", "key", ns)

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
	return fmt.Sprintf("%s/%s", labels[kubeproberv1.LabelKeyProbeNameSpace], labels[kubeproberv1.LabelKeyProbeName])
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProbeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	probePredicates := builder.WithPredicates(&ProbePredicates{})
	probeCronJobPredicates := builder.WithPredicates(&ProbeCronJobPredicates{})

	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeproberv1.Probe{}, probePredicates).
		Owns(&batchv1beta1.CronJob{}, probeCronJobPredicates).
		Complete(r)
}
