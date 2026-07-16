/*
Copyright 2026 Juandi.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Autoscaling defines the configuration for the Horizontal Pod Autoscaler (HPA)
type Autoscaling struct {
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas"`

	// +kubebuilder:default=80
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`
}

// PDBConfig defines the Pod Disruption Budget for high availability
type PDBConfig struct {
	// MaxUnavailable specifies the maximum number of pods that can be unavailable
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// MinAvailable specifies the minimum number of pods that must be available
	// +optional
	MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty"`
}

// AppRoute encapsulates external exposure via Gateway API (HTTPRoute)
type AppRoute struct {
	// +kubebuilder:validation:Required
	Host string `json:"host"`
	// +kubebuilder:default="/"
	Path string `json:"path,omitempty"`
	// GatewayName specifies the target Gateway resource
	// +kubebuilder:default="platform-gateway"
	GatewayName string `json:"gatewayName,omitempty"`
	// GatewayNamespace specifies the namespace of the target Gateway resource
	// +kubebuilder:default="platform-gateway"
	GatewayNamespace string `json:"gatewayNamespace,omitempty"`
}

// ConfigMount defines how and where to mount a ConfigMap or Secret
type ConfigMount struct {
	// SourceName is the name of the existing ConfigMap or Secret in the cluster
	// +kubebuilder:validation:Required
	SourceName string `json:"sourceName"`
	// MountPath is the path within the container (e.g., /etc/config)
	// +kubebuilder:validation:Required
	MountPath string `json:"mountPath"`
}

// Volumes groups configurable volumes
type Volumes struct {
	// +optional
	ConfigMaps []ConfigMount `json:"configMaps,omitempty"`
	// +optional
	Secrets []ConfigMount `json:"secrets,omitempty"`
}

// ProbeConfig defines health checks for the application
type ProbeConfig struct {
	// Liveness probe determines if the container is running properly
	// +optional
	Liveness *corev1.Probe `json:"liveness,omitempty"`

	// Readiness probe determines if the container is ready to accept traffic
	// +optional
	Readiness *corev1.Probe `json:"readiness,omitempty"`
}

// CoreAppSpec defines the desired state of CoreApp
type CoreAppSpec struct {
	// Image defines the container image to run
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Port defines the internal port the application listens on (Service and Pod)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// Autoscaling configures the HPA. It replaces the static replicas field.
	// +kubebuilder:validation:Required
	Autoscaling Autoscaling `json:"autoscaling"`

	// PDB (optional) protects the application during node updates
	// +optional
	PDB *PDBConfig `json:"pdb,omitempty"`

	// Route (optional) exposes the application externally. If nil, it is purely internal.
	// +optional
	Route *AppRoute `json:"route,omitempty"`

	// Probes (optional) configures Liveness and Readiness checks
	// +optional
	Probes *ProbeConfig `json:"probes,omitempty"`

	// Env supports native hardcoded variables and valueFrom (ConfigMaps/Secrets)
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// EnvFrom injects entire ConfigMaps or Secrets as environment variables
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// Volumes allows mounting static files or credentials into the file system
	// +optional
	Volumes *Volumes `json:"volumes,omitempty"`

	// Resources are required for the HPA to function properly
	// +kubebuilder:validation:Required
	Resources corev1.ResourceRequirements `json:"resources"`

	// SecureByDefault will inject a strict SecurityContext (runAsNonRoot, drop ALL caps)
	// +kubebuilder:default=true
	// +optional
	SecureByDefault *bool `json:"secureByDefault,omitempty"`

	// ServiceAccountName binds the Pod to a Kubernetes ServiceAccount.
	// This is critical for Cloud IAM integration (e.g., AWS IRSA or Azure Workload Identity).
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// ImagePullSecrets allows specifying secrets to authenticate against private container registries.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

// CreatedResource represents a resource created and managed by the operator
type CreatedResource struct {
	// Group is the API group of the resource (e.g., "apps" or "" for core)
	Group string `json:"group"`

	// Kind is the type of the resource (e.g., "Deployment", "Service")
	Kind string `json:"kind"`

	// Name is the name of the resource
	Name string `json:"name"`
}

// CoreAppStatus defines the observed state of the system
type CoreAppStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// CreatedResources lists all sub-resources successfully provisioned by the operator.
	// +optional
	CreatedResources []CreatedResource `json:"createdResources,omitempty"`

	// Conditions represent the general state (e.g., Ready, Reconciling, Error)
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="Port",type="integer",JSONPath=".spec.port"
// +kubebuilder:printcolumn:name="MinRep",type="integer",JSONPath=".spec.autoscaling.minReplicas"
// +kubebuilder:printcolumn:name="MaxRep",type="integer",JSONPath=".spec.autoscaling.maxReplicas"
// +kubebuilder:printcolumn:name="Host",type="string",JSONPath=".spec.route.host",priority=1
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// CoreApp is the Schema for the coreapps API
type CoreApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CoreAppSpec   `json:"spec,omitempty"`
	Status CoreAppStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CoreAppList contains a list of CoreApp
type CoreAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CoreApp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CoreApp{}, &CoreAppList{})
}
