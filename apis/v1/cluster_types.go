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

const (
	ExtraCMName = "extra-config"
)

// ClusterSpec defines the desired state of Cluster
type ClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Cluster. Edit cluster_types.go to remove/update
	K8sVersion    string        `json:"k8sVersion,omitempty"`
	ClusterConfig ClusterConfig `json:"clusterConfig,omitempty"`
	ExtraInfo     []ExtraVar    `json:"extraInfo,omitempty"`
}

type ExtraVar struct {
	Name  string `json:"name" protobuf:"bytes,1,opt,name=name"`
	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
}

type ClusterConfig struct {
	Address         string `json:"address"`
	Token           string `json:"token"`
	CACert          string `json:"caCert"`
	CertData        string `json:"certData"`
	KeyData         string `json:"keyData"`
	ProbeNamespaces string `json:"probeNamespaces"`
}

// ClusterStatus defines the observed state of Cluster
type ClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	HeartBeatTimeStamp string          `json:"heartBeatTimeStamp,omitempty"`
	NodeCount          int             `json:"nodeCount,omitempty"`
	AttachedProbes     []string        `json:"attachedProbes,omitempty"`
	Checkers           string          `json:"checkers,omitempty"`
	OnceProbeList      []OnceProbeItem `json:"onceProbeList,omitempty"`
}

type OnceProbeItem struct {
	ID         string   `json:"id,omitempty"`
	CreateTime string   `json:"createTime,omitempty"`
	FinishTime string   `json:"finishTime,omitempty"`
	Probes     []string `json:"probes,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.k8sVersion`
// +kubebuilder:printcolumn:name="NodeCount",type=string,JSONPath=`.status.nodeCount`
// +kubebuilder:printcolumn:name="PROBENAMESPACE",type=string,JSONPath=`.spec.clusterConfig.probeNamespaces`
// +kubebuilder:printcolumn:name="PROBE",type=string,JSONPath=`.status.attachedProbes`
// +kubebuilder:printcolumn:name="TOTAL/ERROR",type=string,JSONPath=`.status.checkers`
// +kubebuilder:printcolumn:name="HEARTBEATTIME",type=string,JSONPath=`.status.heartBeatTimeStamp`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Cluster is the Schema for the clusters API
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
