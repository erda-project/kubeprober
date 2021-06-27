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

package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var clusterlog = logf.Log.WithName("cluster-resource")

func (c *Cluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-kubeprober-erda-cloud-v1-cluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=kubeprober.erda.cloud,resources=clusters,verbs=create;update,versions=v1,name=cluster.probe.kubeprober.erda.cloud,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &Cluster{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (c *Cluster) Default() {
	// TODO(user): fill in your defaulting logic.
}

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-kubeprober-erda-cloud-v1-cluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=kubeprober.erda.cloud,resources=clusters,versions=v1,name=cluster.probe.kubeprober.erda.cloud,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &Cluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (c *Cluster) ValidateCreate() error {
	clusterlog.Info("validate create", "name", c.Name)
	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *Cluster) ValidateUpdate(old runtime.Object) error {
	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *Cluster) ValidateDelete() error {
	clusterlog.Info("validate delete", "name", c.Name)
	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
