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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type CheckerStatus string

const (
	CheckerStatusError   CheckerStatus = "ERROR"
	CheckerStatusWARN    CheckerStatus = "WARN"
	CheckerStatusUNKNOWN CheckerStatus = "UNKNOWN"
	CheckerStatusInfo    CheckerStatus = "INFO"
	CheckerStatusPass    CheckerStatus = "PASS"
)

func (c CheckerStatus) Priority() int {
	if c == CheckerStatusInfo {
		return 1
	} else if c == CheckerStatusUNKNOWN {
		return 2
	} else if c == CheckerStatusWARN {
		return 3
	} else if c == CheckerStatusError {
		return 4
	} else {
		return 0
	}
}

type ProbeCheckerStatus struct {
	// checker name
	Name string `json:"name"`
	// ERROR/WARN/WARN/UNKNOWN
	Status CheckerStatus `json:"status,omitempty"`
	// if not ok, keep error message
	Message string       `json:"message,omitempty"`
	LastRun *metav1.Time `json:"lastRun,omitempty"`
}

type ProbeStatusSpec struct {
	Checkers []ProbeCheckerStatus `json:"checkers,omitempty"`
}

// ProbeStatusStatus defines the observed state of ProbeStatus
type ProbeStatusStates struct {
	Status CheckerStatus `json:"status,omitempty"`
	// if not ok, keep error message
	Message string       `json:"message,omitempty"`
	LastRun *metav1.Time `json:"lastRun,omitempty"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.status"
//+kubebuilder:printcolumn:name="MESSAGE",type="string",JSONPath=".status.message"
//+kubebuilder:printcolumn:name="LASTRUN",type="string",JSONPath=".status.lastRun"
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ProbeStatus is the Schema for the probestatuses API
type ProbeStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProbeStatusSpec   `json:"spec,omitempty"`
	Status ProbeStatusStates `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ProbeStatusList contains a list of ProbeStatus
type ProbeStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProbeStatus `json:"items"`
}

// Client report probe status struct in sdk
type ReportProbeStatusSpec struct {
	ProbeName          string `json:"probeName"`
	ProbeNamespace     string `json:"probeNamespace"`
	ProbeCheckerStatus `json:",inline"`
	Checkers           []ProbeCheckerStatus `json:"checkers"`
}

func init() {
	SchemeBuilder.Register(&ProbeStatus{}, &ProbeStatusList{})
}
