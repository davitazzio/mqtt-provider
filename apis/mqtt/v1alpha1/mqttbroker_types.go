/*
Copyright 2022 The Crossplane Authors.

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
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// MqttBrokerParameters are the configurable fields of a MqttBroker.
type MqttBrokerParameters struct {
	NodeAddress string `json:"nodeAddress"`
	NodePort    string `json:"nodePort"`
	RemoteUser  string `json:"remoteUser"`
}

// MqttBrokerObservation are the observable fields of a MqttBroker.
type MqttBrokerObservation struct {
	QueueState int  `json:"queueState,omitempty"`
	Active     bool `json:"active,omitempty"`
}

// A MqttBrokerSpec defines the desired state of a MqttBroker.
type MqttBrokerSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       MqttBrokerParameters `json:"forProvider"`
}

// A MqttBrokerStatus represents the observed state of a MqttBroker.
type MqttBrokerStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          MqttBrokerObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A MqttBroker is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,mqttprovider}
type MqttBroker struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MqttBrokerSpec   `json:"spec"`
	Status MqttBrokerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MqttBrokerList contains a list of MqttBroker
type MqttBrokerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MqttBroker `json:"items"`
}

// MqttBroker type metadata.
var (
	MqttBrokerKind             = reflect.TypeOf(MqttBroker{}).Name()
	MqttBrokerGroupKind        = schema.GroupKind{Group: Group, Kind: MqttBrokerKind}.String()
	MqttBrokerKindAPIVersion   = MqttBrokerKind + "." + SchemeGroupVersion.String()
	MqttBrokerGroupVersionKind = SchemeGroupVersion.WithKind(MqttBrokerKind)
)

func init() {
	SchemeBuilder.Register(&MqttBroker{}, &MqttBrokerList{})
}
