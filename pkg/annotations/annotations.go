/*
 * Copyright (c) 2018 Mesosphere, Inc
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package annotations

const (
	// Our annotations.
	AnnotationIngressInternal = "dlkb.dcos.io/ingress.internal"
	AnnotationServiceInternal = "dlkb.dcos.io/service.internal"

	// Kubernetes annotations we're interested in.
	KubernetesIngressAllowHttp = "kubernetes.io/ingress.allow-http"
	KubernetesIngressClass     = "kubernetes.io/ingress.class"
	KubernetesIngressForceSSL  = "ingress.kubernetes.io/force-ssl-redirect"
)