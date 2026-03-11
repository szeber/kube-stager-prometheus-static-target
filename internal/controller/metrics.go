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

package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	discoveredJobsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "prometheus_static_target_discovered_scrape_jobs",
			Help: "ScrapeJobs currently included (post-filter) per AdditionalScrapeConfig",
		},
		[]string{"config_name", "config_namespace"},
	)

	filteredJobsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "prometheus_static_target_filtered_scrape_jobs",
			Help: "ScrapeJobs matching labels but excluded by namespace selector per AdditionalScrapeConfig",
		},
		[]string{"config_name", "config_namespace"},
	)

	secretUpdateCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "prometheus_static_target_secret_updates_total",
			Help: "Total number of secret writes (excluding no-op reconciliations)",
		},
		[]string{"config_name", "config_namespace"},
	)

	secretUpdateErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "prometheus_static_target_secret_update_errors_total",
			Help: "Total number of failed secret writes",
		},
		[]string{"config_name", "config_namespace"},
	)

	scrapeJobsLoadedGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "prometheus_static_target_scrape_jobs_loaded",
			Help: "ScrapeJobs loaded (pre-filter) in the last reconciliation per AdditionalScrapeConfig",
		},
		[]string{"config_name", "config_namespace"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		discoveredJobsGauge,
		filteredJobsGauge,
		secretUpdateCounter,
		secretUpdateErrorCounter,
		scrapeJobsLoadedGauge,
	)
}
