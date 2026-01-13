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
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

// Common label keys/values used across kubeprober components.
const (
	LabelKeyApp           = "app"
	LabelValueApp         = "kubeprober.erda.cloud"
	LabelValueProbeMaster = "probe-master"
	LabelValueProbeAgent  = "probe-agent"

	LabelKeyProbeNameSpace = "kubeprober.erda.cloud/probe-namespace"
	LabelKeyProbeName      = "kubeprober.erda.cloud/probe-name"

	LabelKeyCluster   = "kubeprober.erda.cloud/cluster"
	LabelKeyClusterNS = "kubeprober.erda.cloud/cluster-namespace"
	LabelKeyService   = "kubeprober.erda.cloud/service"

	LabelValueServiceSocks5 = "socks5"
)
