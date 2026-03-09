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

package v1

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var clusterlog = logf.Log.WithName("cluster-resource")

func (c *Cluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		WithDefaulter(c).
		WithValidator(c).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-kubeprober-erda-cloud-v1-cluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=kubeprober.erda.cloud,resources=clusters,verbs=create;update,versions=v1,name=cluster.probe.kubeprober.erda.cloud,admissionReviewVersions={v1,v1beta1}

var _ webhook.CustomDefaulter = &Cluster{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type
func (c *Cluster) Default(_ context.Context, obj runtime.Object) error {
	cluster, ok := obj.(*Cluster)
	if ok {
		clusterlog.Info("default", "name", cluster.Name)
	} else {
		clusterlog.Info("default", "unexpectedObject", obj)
	}
	// TODO(user): fill in your defaulting logic.
	return nil
}

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-kubeprober-erda-cloud-v1-cluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=kubeprober.erda.cloud,resources=clusters,versions=v1,name=cluster.probe.kubeprober.erda.cloud,admissionReviewVersions={v1,v1beta1}

var _ webhook.CustomValidator = &Cluster{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (c *Cluster) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	cluster, ok := obj.(*Cluster)
	if ok {
		clusterlog.Info("validate create", "name", cluster.Name)
	} else {
		clusterlog.Info("validate create", "unexpectedObject", obj)
	}
	// TODO(user): fill in your validation logic upon object creation.
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (c *Cluster) ValidateUpdate(_ context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	cluster, ok := newObj.(*Cluster)
	if ok {
		clusterlog.Info("validate update", "name", cluster.Name)
	} else {
		clusterlog.Info("validate update", "unexpectedObject", newObj)
	}
	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (c *Cluster) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	cluster, ok := obj.(*Cluster)
	if ok {
		clusterlog.Info("validate delete", "name", cluster.Name)
	} else {
		clusterlog.Info("validate delete", "unexpectedObject", obj)
	}
	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
