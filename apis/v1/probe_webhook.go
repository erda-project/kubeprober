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
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	clusterRestClient client.Client
	probelog          = logf.Log.WithName("probe-resource")
)

func init() {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		userHomeDir = ""
	}
	kubeConfig := filepath.Join(userHomeDir, ".kube", "config")
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			klog.Errorf("[remote dialer server] get kubernetes client config error: %+v\n", err)
			return
		}
	}

	scheme := runtime.NewScheme()
	AddToScheme(scheme)
	clusterRestClient, err = client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return
	}
}

func (p *Probe) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(p).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-kubeprober-erda-cloud-v1-probe,mutating=true,failurePolicy=fail,sideEffects=None,groups=kubeprober.erda.cloud,resources=probes,verbs=create;update,versions=v1,name=probe.kubeprober.erda.cloud,admissionReviewVersions={v1beta1,v1}

var _ webhook.Defaulter = &Probe{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (p *Probe) Default() {
	probelog.Info("default", "name", p.Name)

	// TODO(user): fill in your defaulting logic.
}

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-kubeprober-erda-cloud-v1-probe,mutating=false,failurePolicy=fail,sideEffects=None,groups=kubeprober.erda.cloud,resources=probes,versions=v1,name=probe.kubeprober.erda.cloud,admissionReviewVersions={v1beta1,v1}

var _ webhook.Validator = &Probe{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (p *Probe) ValidateCreate() error {
	probelog.Info("validate create", "name", p.Name)
	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (p *Probe) ValidateUpdate(old runtime.Object) error {
	probelog.Info("validate update", "name", p.Name)
	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (p *Probe) ValidateDelete() error {
	var err error
	var attachedCluster []string
	probelog.Info("validate delete", "name", p.Name)
	clusters := &ClusterList{}
	if err = clusterRestClient.List(context.Background(), clusters); err != nil {
		return nil
	}

	for i := range clusters.Items {
		cluster := clusters.Items[i]
		for k, v := range cluster.GetLabels() {
			if v == "true" && strings.Split(k, "/")[0] == "probe" && strings.Split(k, "/")[1] == p.Name {
				attachedCluster = append(attachedCluster, cluster.Name)
			}
		}
	}

	if len(attachedCluster) > 0 {
		errstr := fmt.Sprintf("There are cluster %s attached this probe, you need detached cluster first", attachedCluster)
		return errors.New(errstr)
	}
	return nil
}
