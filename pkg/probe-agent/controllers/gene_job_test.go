package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
)

func TestGeneJob(t *testing.T) {
	pj := kubeproberv1.Probe{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-probe",
			Namespace: "test-namespace",
		},
		Spec: kubeproberv1.ProbeSpec{
			Template: apiv1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "",
						Image: "",
						Env: []corev1.EnvVar{
							{
								Name:  "env_test1",
								Value: "env_value1",
							},
						},
						EnvFrom: []corev1.EnvFromSource{
							{
								ConfigMapRef: &corev1.ConfigMapEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "envFromSource_test1",
									},
								},
							},
						},
					},
				},
			},
			Configs: []kubeproberv1.Config{
				{
					Name: "config-test",
					Env: []corev1.EnvVar{
						{
							Name:  "env_test2",
							Value: "env_value2",
						},
					},
				},
			},
		},
	}
	job, err := genJob(&pj)
	assert.NoError(t, err)
	t.Logf("job: %+v", job)
}
