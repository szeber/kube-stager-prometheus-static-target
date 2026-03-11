package controller

import (
	"context"
	"fmt"
	"testing"

	prometheusv1 "github.com/szeber/kube-stager-prometheus-static-target/api/v1"
	"github.com/szeber/kube-stager-prometheus-static-target/internal/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// --- processTargets tests ---

func TestProcessTargets_FiltersNamespace(t *testing.T) {
	r := &AdditionalScrapeConfigReconciler{}
	config := &prometheusv1.AdditionalScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: prometheusv1.AdditionalScrapeConfigSpec{
			ScrapeJobNamespaceSelector: prometheusv1.NamespaceSelector{
				MatchNames: []string{"ns1"},
			},
		},
	}
	targets := &prometheusv1.ScrapeJobList{
		Items: []prometheusv1.ScrapeJob{
			{ObjectMeta: metav1.ObjectMeta{Name: "j1", Namespace: "ns1"}, Spec: prometheusv1.ScrapeJobSpec{JobName: "job1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "j2", Namespace: "ns2"}, Spec: prometheusv1.ScrapeJobSpec{JobName: "job2"}},
		},
	}

	discovered, jobs := r.processTargets(config, targets)
	if len(discovered) != 1 || discovered[0] != "ns1/j1" {
		t.Errorf("discovered = %v, want [ns1/j1]", discovered)
	}
	if len(jobs) != 1 || jobs[0].JobName != "job1" {
		t.Errorf("jobs = %v, want [job1]", jobs)
	}
}

func TestProcessTargets_SortsDiscovered(t *testing.T) {
	r := &AdditionalScrapeConfigReconciler{}
	config := &prometheusv1.AdditionalScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: prometheusv1.AdditionalScrapeConfigSpec{
			ScrapeJobNamespaceSelector: prometheusv1.NamespaceSelector{Any: true},
		},
	}
	targets := &prometheusv1.ScrapeJobList{
		Items: []prometheusv1.ScrapeJob{
			{ObjectMeta: metav1.ObjectMeta{Name: "beta", Namespace: "ns1"}, Spec: prometheusv1.ScrapeJobSpec{JobName: "beta"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "alpha", Namespace: "ns1"}, Spec: prometheusv1.ScrapeJobSpec{JobName: "alpha"}},
		},
	}

	discovered, _ := r.processTargets(config, targets)
	if len(discovered) != 2 || discovered[0] != "ns1/alpha" || discovered[1] != "ns1/beta" {
		t.Errorf("discovered = %v, want [ns1/alpha ns1/beta]", discovered)
	}
}

func TestProcessTargets_EmptyInput(t *testing.T) {
	r := &AdditionalScrapeConfigReconciler{}
	config := &prometheusv1.AdditionalScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: prometheusv1.AdditionalScrapeConfigSpec{
			ScrapeJobNamespaceSelector: prometheusv1.NamespaceSelector{Any: true},
		},
	}
	targets := &prometheusv1.ScrapeJobList{}

	discovered, jobs := r.processTargets(config, targets)
	if discovered != nil {
		t.Errorf("discovered = %v, want nil", discovered)
	}
	if jobs != nil {
		t.Errorf("jobs = %v, want nil", jobs)
	}
}

func TestProcessTargets_MultipleStaticConfigs(t *testing.T) {
	r := &AdditionalScrapeConfigReconciler{}
	config := &prometheusv1.AdditionalScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: prometheusv1.AdditionalScrapeConfigSpec{
			ScrapeJobNamespaceSelector: prometheusv1.NamespaceSelector{Any: true},
		},
	}
	targets := &prometheusv1.ScrapeJobList{
		Items: []prometheusv1.ScrapeJob{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "multi", Namespace: "ns1"},
				Spec: prometheusv1.ScrapeJobSpec{
					JobName: "multi-job",
					StaticConfigs: []prometheusv1.ScrapeJobStaticConfig{
						{Targets: []string{"host1:9090"}, Labels: map[string]string{"env": "prod"}},
						{Targets: []string{"host2:9090"}, Labels: map[string]string{"env": "staging"}},
					},
				},
			},
		},
	}

	_, jobs := r.processTargets(config, targets)
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(jobs))
	}
	if len(jobs[0].StaticConfigs) != 2 {
		t.Errorf("got %d static configs, want 2", len(jobs[0].StaticConfigs))
	}
	assertStaticConfig(t, jobs[0].StaticConfigs[0], []string{"host1:9090"}, "prod")
	assertStaticConfig(t, jobs[0].StaticConfigs[1], []string{"host2:9090"}, "staging")
}

func assertStaticConfig(t *testing.T, sc prometheus.StaticConfig, expectedTargets []string, expectedEnv string) {
	t.Helper()
	if len(sc.Targets) != len(expectedTargets) || sc.Targets[0] != expectedTargets[0] {
		t.Errorf("targets = %v, want %v", sc.Targets, expectedTargets)
	}
	if sc.Labels["env"] != expectedEnv {
		t.Errorf("labels[env] = %q, want %q", sc.Labels["env"], expectedEnv)
	}
}

// --- findConfigsForSecret tests ---

func TestFindConfigsForSecret_Match(t *testing.T) {
	configs := &prometheusv1.AdditionalScrapeConfigList{
		Items: []prometheusv1.AdditionalScrapeConfig{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "cfg1", Namespace: "default"},
				Spec: prometheusv1.AdditionalScrapeConfigSpec{
					SecretName:      "my-secret",
					SecretNamespace: "default",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "cfg2", Namespace: "other"},
				Spec: prometheusv1.AdditionalScrapeConfigSpec{
					SecretName:      "my-secret",
					SecretNamespace: "other-ns",
				},
			},
		},
	}

	r := &AdditionalScrapeConfigReconciler{
		KubeClient: &mockKubeClient{configs: configs},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
	}
	requests := r.findConfigsForSecret(context.Background(), secret)
	if len(requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(requests))
	}
	if requests[0].Name != "cfg1" || requests[0].Namespace != "default" {
		t.Errorf("request = %v, want default/cfg1", requests[0].NamespacedName)
	}
}

func TestFindConfigsForSecret_Error(t *testing.T) {
	r := &AdditionalScrapeConfigReconciler{
		KubeClient: &mockKubeClient{err: fmt.Errorf("api error")},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "any", Namespace: "default"},
	}
	requests := r.findConfigsForSecret(context.Background(), secret)
	if len(requests) != 0 {
		t.Errorf("expected empty requests on error, got %d", len(requests))
	}
}

// --- findConfigsForJobs tests ---

func TestFindConfigsForJobs_LabelMatch(t *testing.T) {
	allConfigs := &prometheusv1.AdditionalScrapeConfigList{
		Items: []prometheusv1.AdditionalScrapeConfig{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "cfg1", Namespace: "default"},
				Spec: prometheusv1.AdditionalScrapeConfigSpec{
					ScrapeJobLabels:            map[string]string{"app": "web"},
					ScrapeJobNamespaceSelector: prometheusv1.NamespaceSelector{Any: true},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "cfg2", Namespace: "default"},
				Spec: prometheusv1.AdditionalScrapeConfigSpec{
					ScrapeJobLabels:            map[string]string{"app": "api"},
					ScrapeJobNamespaceSelector: prometheusv1.NamespaceSelector{Any: true},
				},
			},
		},
	}

	r := &AdditionalScrapeConfigReconciler{
		KubeClient: &mockKubeClient{allConfigs: allConfigs},
	}

	job := &prometheusv1.ScrapeJob{
		ObjectMeta: metav1.ObjectMeta{Name: "j1", Namespace: "ns1", Labels: map[string]string{"app": "web"}},
	}
	requests := r.findConfigsForJobs(context.Background(), job)
	if len(requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(requests))
	}
	if requests[0].Name != "cfg1" {
		t.Errorf("request name = %q, want cfg1", requests[0].Name)
	}
}

func TestFindConfigsForJobs_EmptyLabelsSkipped(t *testing.T) {
	allConfigs := &prometheusv1.AdditionalScrapeConfigList{
		Items: []prometheusv1.AdditionalScrapeConfig{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "cfg-no-labels", Namespace: "default"},
				Spec: prometheusv1.AdditionalScrapeConfigSpec{
					ScrapeJobLabels:            map[string]string{},
					ScrapeJobNamespaceSelector: prometheusv1.NamespaceSelector{Any: true},
				},
			},
		},
	}

	r := &AdditionalScrapeConfigReconciler{
		KubeClient: &mockKubeClient{allConfigs: allConfigs},
	}

	job := &prometheusv1.ScrapeJob{
		ObjectMeta: metav1.ObjectMeta{Name: "j1", Namespace: "ns1", Labels: map[string]string{"app": "web"}},
	}
	requests := r.findConfigsForJobs(context.Background(), job)
	if len(requests) != 0 {
		t.Errorf("expected 0 requests for empty labels config, got %d", len(requests))
	}
}

func TestFindConfigsForJobs_NamespaceFiltering(t *testing.T) {
	allConfigs := &prometheusv1.AdditionalScrapeConfigList{
		Items: []prometheusv1.AdditionalScrapeConfig{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "cfg1", Namespace: "default"},
				Spec: prometheusv1.AdditionalScrapeConfigSpec{
					ScrapeJobLabels: map[string]string{"app": "web"},
					ScrapeJobNamespaceSelector: prometheusv1.NamespaceSelector{
						MatchNames: []string{"ns-allowed"},
					},
				},
			},
		},
	}

	r := &AdditionalScrapeConfigReconciler{
		KubeClient: &mockKubeClient{allConfigs: allConfigs},
	}

	// Job in wrong namespace
	job := &prometheusv1.ScrapeJob{
		ObjectMeta: metav1.ObjectMeta{Name: "j1", Namespace: "ns-other", Labels: map[string]string{"app": "web"}},
	}
	requests := r.findConfigsForJobs(context.Background(), job)
	if len(requests) != 0 {
		t.Errorf("expected 0 requests for wrong namespace, got %d", len(requests))
	}

	// Job in correct namespace
	job.Namespace = "ns-allowed"
	requests = r.findConfigsForJobs(context.Background(), job)
	if len(requests) != 1 {
		t.Errorf("expected 1 request for correct namespace, got %d", len(requests))
	}
}

func TestFindConfigsForJobs_Error(t *testing.T) {
	r := &AdditionalScrapeConfigReconciler{
		KubeClient: &mockKubeClient{err: fmt.Errorf("api error")},
	}
	job := &prometheusv1.ScrapeJob{
		ObjectMeta: metav1.ObjectMeta{Name: "j1", Namespace: "ns1"},
	}
	requests := r.findConfigsForJobs(context.Background(), job)
	if len(requests) != 0 {
		t.Errorf("expected empty requests on error, got %d", len(requests))
	}
}
