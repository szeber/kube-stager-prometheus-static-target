/*
Copyright 2023.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AdditionalScrapeConfigSpec defines the desired state of AdditionalScrapeConfig
type AdditionalScrapeConfigSpec struct {
	SecretName                 string            `json:"secretName"`
	SecretNamespace            string            `json:"secretNamespace"`
	SecretKey                  string            `json:"secretKey"`
	ScrapeJobLabels            map[string]string `json:"scrapeJobLabels,omitempty"`
	ScrapeJobNamespaceSelector NamespaceSelector `json:"scrapeJobNamespaceSelector,omitempty"`
}

// AdditionalScrapeConfigStatus defines the observed state of AdditionalScrapeConfig
type AdditionalScrapeConfigStatus struct {
	DiscoveredScrapeJobs []string `json:"discoveredScrapeJobs"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AdditionalScrapeConfig is the Schema for the additionalscrapeconfigs API
type AdditionalScrapeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AdditionalScrapeConfigSpec   `json:"spec,omitempty"`
	Status AdditionalScrapeConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AdditionalScrapeConfigList contains a list of AdditionalScrapeConfig
type AdditionalScrapeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AdditionalScrapeConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AdditionalScrapeConfig{}, &AdditionalScrapeConfigList{})
}
