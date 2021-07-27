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
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ProbeNamespace       = "KUBEPROBER_PROBE_NAMESPACE"
	ProbeName            = "KUBEPROBER_PROBE_NAME"
	ProbeStatusReportUrl = "KUBEPROBER_STATUS_REPORT_URL"

	LabelKeyApp            = "app"
	LabelValueApp          = "kubeprober.erda.cloud"
	LabelKeyProbeNameSpace = "kubeprober.erda.cloud/probe-namespace"
	LabelKeyProbeName      = "kubeprober.erda.cloud/probe-name"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type Policy struct {
	// unit: minute
	RunInterval int `json:"runInterval,omitempty"`
}

// ProbeSpec defines the desired state of Probe
type ProbeSpec struct {
	Policy   Policy        `json:"policy,omitempty"`
	Template apiv1.PodSpec `json:"template,omitempty"`
	Configs  []Config      `json:"configs,omitempty"`
}

// Checker defines the desired state of Checker
type Config struct {
	Name string         `json:"name,omitempty"`
	Env  []apiv1.EnvVar `json:"env,omitempty"`
}

// ProbeStatus defines the observed state of Probe
type ProbeStates struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	MD5 string `json:"md5,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="RUNINTERVAL",type="integer",JSONPath=".spec.policy.runInterval"
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Probe is the Schema for the probes API
type Probe struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProbeSpec   `json:"spec,omitempty"`
	Status ProbeStates `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ProbeList contains a list of Probe
type ProbeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Probe `json:"items,omitempty"`
}

func init() {
	SchemeBuilder.Register(&Probe{}, &ProbeList{})
}
