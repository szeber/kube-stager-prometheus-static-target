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
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/szeber/kube-stager-prometheus-static-target/internal/kubernetes"
	"github.com/szeber/kube-stager-prometheus-static-target/internal/prometheus"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sort"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	prometheusv1 "github.com/szeber/kube-stager-prometheus-static-target/api/v1"
)

// AdditionalScrapeConfigReconciler reconciles a AdditionalScrapeConfig object
type AdditionalScrapeConfigReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	KubeClient *kubernetes.Client
}

//+kubebuilder:rbac:groups=prometheus-static-target.kube-stager.io,resources=additionalscrapeconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=prometheus-static-target.kube-stager.io,resources=additionalscrapeconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=prometheus-static-target.kube-stager.io,resources=additionalscrapeconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=prometheus-static-target.kube-stager.io,resources=scrapejobs,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update

func (r *AdditionalScrapeConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info(fmt.Sprintf("Reconciling %s/%s", req.Namespace, req.Name))

	configYaml, err := r.KubeClient.GetAdditionalScrapeConfig(ctx, req.Namespace, req.Name)
	if nil != err {
		return ctrl.Result{}, err
	}

	targetList, err := r.loadTargets(ctx, logger, configYaml)
	if nil != err {
		return ctrl.Result{}, err
	}

	discoveredTargets, jobs := r.processTargets(configYaml, targetList)

	if err = r.updateStatusIfNeeded(ctx, discoveredTargets, configYaml); nil != err {
		return ctrl.Result{}, err
	}

	err = r.updateSecret(ctx, logger, configYaml, jobs)

	return ctrl.Result{}, err
}

func (r *AdditionalScrapeConfigReconciler) loadTargets(ctx context.Context, logger logr.Logger, config *prometheusv1.AdditionalScrapeConfig) (*prometheusv1.ScrapeJobList, error) {
	logger.Info("Loading targets")
	targetList, err := r.KubeClient.LoadScrapeJobs(ctx, config)

	if nil != err {
		return nil, err
	}

	logger.Info(fmt.Sprintf("Loaded %d targets matching the labels", len(targetList.Items)))

	return targetList, err
}

func (r *AdditionalScrapeConfigReconciler) processTargets(config *prometheusv1.AdditionalScrapeConfig, targetList *prometheusv1.ScrapeJobList) ([]string, []prometheus.Job) {
	var discoveredJobs []string
	var jobs []prometheus.Job
	for _, target := range targetList.Items {
		if !config.Spec.ScrapeJobNamespaceSelector.Matches(target.Namespace, config.Namespace) {
			continue
		}
		discoveredJobs = append(discoveredJobs, fmt.Sprintf("%s/%s", target.Namespace, target.Name))
		job := prometheus.Job{
			JobName:       target.Spec.JobName,
			StaticConfigs: []prometheus.StaticConfig{},
		}
		for _, staticConfig := range target.Spec.StaticConfigs {
			job.StaticConfigs = append(job.StaticConfigs, prometheus.StaticConfig{
				Targets: staticConfig.Targets,
				Labels:  staticConfig.Labels,
			})
		}

		jobs = append(jobs, job)
	}

	sort.Strings(discoveredJobs)

	return discoveredJobs, jobs
}

func (r *AdditionalScrapeConfigReconciler) updateStatusIfNeeded(ctx context.Context, discoveredTargets []string, config *prometheusv1.AdditionalScrapeConfig) error {
	if !reflect.DeepEqual(discoveredTargets, config.Status.DiscoveredScrapeJobs) {
		config.Status.DiscoveredScrapeJobs = discoveredTargets
		return r.Status().Update(ctx, config)
	}

	return nil
}

func (r *AdditionalScrapeConfigReconciler) updateSecret(ctx context.Context, logger logr.Logger, config *prometheusv1.AdditionalScrapeConfig, jobs []prometheus.Job) error {
	secret, secretExists, err := r.KubeClient.GetSecret(ctx, config)
	if nil != err {
		return err
	}

	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].JobName < jobs[j].JobName
	})

	yamlData, err := yaml.Marshal(jobs)
	if nil != err {
		return err
	}

	if nil == secret.Data {
		secret.Data = make(map[string][]byte)
	}

	if secretExists && string(secret.Data[config.Spec.SecretKey]) == string(yamlData) {
		return nil
	}

	logger.Info("Updating secret")
	secret.Data[config.Spec.SecretKey] = yamlData
	logger.Info(fmt.Sprintf("Updating secret to %+v", secret.Data))

	return r.KubeClient.CreateOrUpdateSecret(ctx, secretExists, secret)
}

func (r *AdditionalScrapeConfigReconciler) findConfigsForSecret(secret client.Object) []reconcile.Request {
	configYamlList, err := r.KubeClient.FindAdditionalScrapeConfigsForSecret(secret)
	if err != nil {
		return []reconcile.Request{}
	}

	var requests []reconcile.Request
	for _, item := range configYamlList.Items {
		if item.Spec.SecretNamespace == secret.GetNamespace() {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				},
			})
		}
	}

	return requests
}

func (r *AdditionalScrapeConfigReconciler) findConfigsForJobs(target client.Object) []reconcile.Request {
	allConfigYamls, err := r.KubeClient.GetAllAdditionalScrapeConfigs()
	if err != nil {
		return []reconcile.Request{}
	}

	targetLabels := target.GetLabels()

	var requests []reconcile.Request
	for _, item := range allConfigYamls.Items {
		if !item.Spec.ScrapeJobNamespaceSelector.Matches(target.GetNamespace(), item.GetName()) {
			continue
		}

		if len(item.Spec.ScrapeJobLabels) == 0 {
			continue
		}

		for key, value := range item.Spec.ScrapeJobLabels {
			if targetLabels[key] != value {
				continue
			}
		}

		requests = append(
			requests,
			reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				},
			},
		)
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *AdditionalScrapeConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.KubeClient == nil {
		r.KubeClient = kubernetes.NewClient(r.Client)
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &prometheusv1.AdditionalScrapeConfig{}, ".spec.secretName", func(rawObj client.Object) []string {
			// grab the config object, extract the short name.
			config := rawObj.(*prometheusv1.AdditionalScrapeConfig)
			return []string{config.Spec.SecretName}
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&prometheusv1.AdditionalScrapeConfig{}).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(r.findConfigsForSecret),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(
			&source.Kind{Type: &prometheusv1.ScrapeJob{}},
			handler.EnqueueRequestsFromMapFunc(r.findConfigsForJobs),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}
