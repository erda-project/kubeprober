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

// Common annotation keys used across kubeprober components.
const (
	AnnotationSocks5Enable      = "kubeprober.erda.cloud/socks5-enable"
	AnnotationSocks5Port        = "kubeprober.erda.cloud/socks5-port"
	AnnotationSocks5BoundPort   = "kubeprober.erda.cloud/socks5-bound-port"
	AnnotationSocks5ServiceName = "kubeprober.erda.cloud/socks5-service"
)
