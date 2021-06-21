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

package v1alpha1

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ProbeNamespace       = "KUBEPROBER_PROBE_NAMESPACE"
	ProbeName            = "KUBEPROBER_PROBE_NAME"
	ProbeItemName        = "KUBEPROBER_PROBE_ITEM_NAME"
	ProbeStatusReportUrl = "KUBEPROBER_STATUS_REPORT_URL"

	LabelKeyProbeNameSpace = "kubeprober.erda.cloud/probe-namespace"
	LabelKeyProbeName      = "kubeprober.erda.cloud/probe-name"
	LabelKeyProbeItemName  = "kubeprober.erda.cloud/probe-item-name"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ProbeItem struct {
	// prob item type: golang/shell/...
	Name string        `json:"name,omitempty"`
	Type string        `json:"type,omitempty"`
	Spec apiv1.PodSpec `json:"spec,omitempty"`
}

type Policy struct {
	// unit: minute
	RunInterval int `json:"runInterval,omitempty"`
}

// ProbeSpec defines the desired state of Probe
type ProbeSpec struct {
	ProbeList []ProbeItem `json:"probeList,omitempty"`
	Policy    Policy      `json:"policy,omitempty"`
}

// ProbeStatus defines the observed state of Probe
type ProbeStates struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

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
