package controllers

import (
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type JobOp func(*batchv1.Job)
type JobSpecOp func(spec *batchv1.JobSpec)

// Job : build job with job ops
func Job(name string, ops ...JobOp) *batchv1.Job {
	j := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	for _, op := range ops {
		op(j)
	}

	return j
}

// JobNamespace : render job with namespace
func JobNamespace(namespace string) JobOp {
	return func(job *batchv1.Job) {
		job.ObjectMeta.Namespace = namespace
	}
}

// JobLabels : render job with labels
func JobLabels(labels map[string]string) JobOp {
	return func(job *batchv1.Job) {
		if job.Labels == nil {
			job.Labels = make(map[string]string)
		}
		for k, v := range labels {
			job.Labels[k] = v
		}
	}
}

func JobOwnerReference(owns []metav1.OwnerReference) JobOp {
	return func(job *batchv1.Job) {
		if job.OwnerReferences == nil {
			job.OwnerReferences = make([]metav1.OwnerReference, 0)
		}
		for i := range owns {
			job.OwnerReferences = append(job.OwnerReferences, owns[i])
		}
	}
}

// JobSpec : render job with job spec
func JobSpec(ops ...JobSpecOp) JobOp {
	return func(job *batchv1.Job) {
		js := &job.Spec

		for _, op := range ops {
			op(js)
		}

		job.Spec = *js
	}
}

func JobSpecActiveDeadlineSeconds(t int64) JobSpecOp {
	return func(spec *batchv1.JobSpec) {
		spec.ActiveDeadlineSeconds = &t
	}
}

func JobSpecBackoffLimit(l int32) JobSpecOp {
	return func(spec *batchv1.JobSpec) {
		spec.BackoffLimit = &l
	}
}

func JobSpecTmpLabels(labels map[string]string) JobSpecOp {
	return func(spec *batchv1.JobSpec) {
		if spec.Template.ObjectMeta.Labels == nil {
			spec.Template.ObjectMeta.Labels = make(map[string]string)
		}
		for k, v := range labels {
			spec.Template.ObjectMeta.Labels[k] = v
		}
	}
}

func JobSpecTmpPod(podSpec apiv1.PodSpec) JobSpecOp {
	return func(spec *batchv1.JobSpec) {
		spec.Template.Spec = podSpec
	}
}

func JobSpecTmpRestartPolicy(p corev1.RestartPolicy) JobSpecOp {
	return func(spec *batchv1.JobSpec) {
		spec.Template.Spec.RestartPolicy = p
	}
}

func JobSpecTmpServiceAccount(a string) JobSpecOp {
	return func(spec *batchv1.JobSpec) {
		spec.Template.Spec.ServiceAccountName = a
	}
}

func JobSpecTmpImagePullPolicy(p corev1.PullPolicy) JobSpecOp {
	return func(spec *batchv1.JobSpec) {
		for i := range spec.Template.Spec.Containers {
			spec.Template.Spec.Containers[i].ImagePullPolicy = p
		}
	}
}

func JobSpecTmpPodEnvs(envs []corev1.EnvVar) JobSpecOp {
	return func(spec *batchv1.JobSpec) {
		for i := range spec.Template.Spec.Containers {
			c := &spec.Template.Spec.Containers[i]
			if c.Env == nil {
				c.Env = make([]corev1.EnvVar, 0)
				c.Env = append(c.Env, envs...)
			}
		}
	}
}

func JobSpecTmpPodEnvSources(s []corev1.EnvFromSource) JobSpecOp {
	return func(spec *batchv1.JobSpec) {
		for i := range spec.Template.Spec.Containers {
			c := &spec.Template.Spec.Containers[i]
			if c.EnvFrom == nil {
				c.EnvFrom = make([]corev1.EnvFromSource, 0)
				c.EnvFrom = append(c.EnvFrom, s...)
			}
		}
	}
}
