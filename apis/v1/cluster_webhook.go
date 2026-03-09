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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

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
	if _, ok := obj.(*Cluster); !ok {
		return fmt.Errorf("expected *Cluster, got %T", obj)
	}
	return nil
}

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-kubeprober-erda-cloud-v1-cluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=kubeprober.erda.cloud,resources=clusters,versions=v1,name=cluster.probe.kubeprober.erda.cloud,admissionReviewVersions={v1,v1beta1}

var _ webhook.CustomValidator = &Cluster{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (c *Cluster) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	if _, ok := obj.(*Cluster); !ok {
		return nil, fmt.Errorf("expected *Cluster, got %T", obj)
	}
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (c *Cluster) ValidateUpdate(_ context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	if _, ok := newObj.(*Cluster); !ok {
		return nil, fmt.Errorf("expected *Cluster, got %T", newObj)
	}
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (c *Cluster) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	if _, ok := obj.(*Cluster); !ok {
		return nil, fmt.Errorf("expected *Cluster, got %T", obj)
	}
	return nil, nil
}
