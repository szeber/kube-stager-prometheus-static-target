package controller

import (
	"context"
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	prometheusv1 "github.com/szeber/kube-stager-prometheus-static-target/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// gaugeLabelSetExists checks whether a GaugeVec contains a label set matching
// the given label name-value pairs, without creating a new series (unlike
// WithLabelValues). Labels are matched by name, not by position.
func gaugeLabelSetExists(vec *prometheus.GaugeVec, labelValues map[string]string) bool {
	ch := make(chan prometheus.Metric, 100)
	vec.Collect(ch)
	close(ch)
	for m := range ch {
		metric := &dto.Metric{}
		if err := m.Write(metric); err != nil {
			continue
		}
		if len(metric.Label) != len(labelValues) {
			continue
		}
		match := true
		for _, lp := range metric.Label {
			if v, ok := labelValues[lp.GetName()]; !ok || v != lp.GetValue() {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestProcessTargets_SetsGaugeMetrics(t *testing.T) {
	r := &AdditionalScrapeConfigReconciler{}
	config := &prometheusv1.AdditionalScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg-gauge", Namespace: "ns-gauge"},
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
			{ObjectMeta: metav1.ObjectMeta{Name: "j3", Namespace: "ns3"}, Spec: prometheusv1.ScrapeJobSpec{JobName: "job3"}},
		},
	}

	r.processTargets(config, targets)

	discovered := testutil.ToFloat64(discoveredJobsGauge.WithLabelValues("cfg-gauge", "ns-gauge"))
	if discovered != 1 {
		t.Errorf("discovered gauge = %v, want 1", discovered)
	}

	filtered := testutil.ToFloat64(filteredJobsGauge.WithLabelValues("cfg-gauge", "ns-gauge"))
	if filtered != 2 {
		t.Errorf("filtered gauge = %v, want 2", filtered)
	}
}

func TestProcessTargets_AllDiscovered(t *testing.T) {
	r := &AdditionalScrapeConfigReconciler{}
	config := &prometheusv1.AdditionalScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg-all", Namespace: "ns-all"},
		Spec: prometheusv1.AdditionalScrapeConfigSpec{
			ScrapeJobNamespaceSelector: prometheusv1.NamespaceSelector{Any: true},
		},
	}
	targets := &prometheusv1.ScrapeJobList{
		Items: []prometheusv1.ScrapeJob{
			{ObjectMeta: metav1.ObjectMeta{Name: "j1", Namespace: "ns1"}, Spec: prometheusv1.ScrapeJobSpec{JobName: "job1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "j2", Namespace: "ns2"}, Spec: prometheusv1.ScrapeJobSpec{JobName: "job2"}},
		},
	}

	r.processTargets(config, targets)

	discovered := testutil.ToFloat64(discoveredJobsGauge.WithLabelValues("cfg-all", "ns-all"))
	if discovered != 2 {
		t.Errorf("discovered gauge = %v, want 2", discovered)
	}

	filtered := testutil.ToFloat64(filteredJobsGauge.WithLabelValues("cfg-all", "ns-all"))
	if filtered != 0 {
		t.Errorf("filtered gauge = %v, want 0", filtered)
	}
}

func TestUpdateSecret_IncrementsCounterOnSuccess(t *testing.T) {
	logger := zap.New(zap.UseDevMode(true))
	mock := &mockKubeClient{
		secret:       &corev1.Secret{Data: map[string][]byte{}},
		secretExists: false,
	}
	r := &AdditionalScrapeConfigReconciler{KubeClient: mock}
	config := &prometheusv1.AdditionalScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg-counter", Namespace: "ns-counter"},
		Spec: prometheusv1.AdditionalScrapeConfigSpec{
			SecretName:      "my-secret",
			SecretNamespace: "secret-ns",
			SecretKey:       "key",
		},
	}

	before := testutil.ToFloat64(secretUpdateCounter.WithLabelValues("cfg-counter", "ns-counter"))

	err := r.updateSecret(context.Background(), logger, config, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after := testutil.ToFloat64(secretUpdateCounter.WithLabelValues("cfg-counter", "ns-counter"))
	if after-before != 1 {
		t.Errorf("secret update counter increment = %v, want 1", after-before)
	}
}

func TestUpdateSecret_IncrementsErrorCounterOnFailure(t *testing.T) {
	logger := zap.New(zap.UseDevMode(true))
	mock := &mockKubeClient{
		secret:       &corev1.Secret{Data: map[string][]byte{}},
		secretExists: false,
		createUpdateFn: func(_ context.Context, _ bool, _ *corev1.Secret) error {
			return fmt.Errorf("write failed")
		},
	}
	r := &AdditionalScrapeConfigReconciler{KubeClient: mock}
	config := &prometheusv1.AdditionalScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg-err", Namespace: "ns-err"},
		Spec: prometheusv1.AdditionalScrapeConfigSpec{
			SecretName:      "err-secret",
			SecretNamespace: "err-ns",
			SecretKey:       "key",
		},
	}

	before := testutil.ToFloat64(secretUpdateErrorCounter.WithLabelValues("cfg-err", "ns-err"))

	err := r.updateSecret(context.Background(), logger, config, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	after := testutil.ToFloat64(secretUpdateErrorCounter.WithLabelValues("cfg-err", "ns-err"))
	if after-before != 1 {
		t.Errorf("secret update error counter increment = %v, want 1", after-before)
	}

	successCount := testutil.ToFloat64(secretUpdateCounter.WithLabelValues("cfg-err", "ns-err"))
	if successCount != 0 {
		t.Errorf("secret update counter = %v, want 0 (should not increment on error)", successCount)
	}
}

func TestUpdateSecret_NoOpSkipsCounters(t *testing.T) {
	logger := zap.New(zap.UseDevMode(true))
	existingData := []byte("- jobname: test\n  static_configs: []\n")
	mock := &mockKubeClient{
		secret: &corev1.Secret{
			Data: map[string][]byte{"key": existingData},
		},
		secretExists: true,
	}
	r := &AdditionalScrapeConfigReconciler{KubeClient: mock}
	config := &prometheusv1.AdditionalScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg-noop", Namespace: "ns-noop"},
		Spec: prometheusv1.AdditionalScrapeConfigSpec{
			SecretName:      "noop-secret",
			SecretNamespace: "noop-ns",
			SecretKey:       "key",
		},
	}

	err := r.updateSecret(context.Background(), logger, config, nil)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}

	// Update mock to return the data that was just written so next call is a no-op
	mock.secret.Data["key"] = []byte("[]\n")

	beforeSuccess := testutil.ToFloat64(secretUpdateCounter.WithLabelValues("cfg-noop", "ns-noop"))
	beforeError := testutil.ToFloat64(secretUpdateErrorCounter.WithLabelValues("cfg-noop", "ns-noop"))

	err = r.updateSecret(context.Background(), logger, config, nil)
	if err != nil {
		t.Fatalf("unexpected error on no-op call: %v", err)
	}

	afterSuccess := testutil.ToFloat64(secretUpdateCounter.WithLabelValues("cfg-noop", "ns-noop"))
	afterError := testutil.ToFloat64(secretUpdateErrorCounter.WithLabelValues("cfg-noop", "ns-noop"))

	if afterSuccess != beforeSuccess {
		t.Errorf("success counter changed on no-op: before=%v, after=%v", beforeSuccess, afterSuccess)
	}
	if afterError != beforeError {
		t.Errorf("error counter changed on no-op: before=%v, after=%v", beforeError, afterError)
	}
}

func TestLoadTargets_SetsScrapeJobsLoadedGauge(t *testing.T) {
	logger := zap.New(zap.UseDevMode(true))
	mock := &mockKubeClient{
		scrapeJobs: &prometheusv1.ScrapeJobList{
			Items: []prometheusv1.ScrapeJob{
				{ObjectMeta: metav1.ObjectMeta{Name: "j1", Namespace: "ns1"}, Spec: prometheusv1.ScrapeJobSpec{JobName: "job1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "j2", Namespace: "ns2"}, Spec: prometheusv1.ScrapeJobSpec{JobName: "job2"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "j3", Namespace: "ns3"}, Spec: prometheusv1.ScrapeJobSpec{JobName: "job3"}},
			},
		},
	}
	r := &AdditionalScrapeConfigReconciler{KubeClient: mock}
	config := &prometheusv1.AdditionalScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg-loaded", Namespace: "ns-loaded"},
	}

	_, err := r.loadTargets(context.Background(), logger, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded := testutil.ToFloat64(scrapeJobsLoadedGauge.WithLabelValues("cfg-loaded", "ns-loaded"))
	if loaded != 3 {
		t.Errorf("scrape jobs loaded gauge = %v, want 3", loaded)
	}
}

func TestLoadTargets_ErrorDoesNotSetGauge(t *testing.T) {
	// Pre-set the gauge to a sentinel value to detect unwanted changes
	scrapeJobsLoadedGauge.WithLabelValues("cfg-err-load", "ns-err-load").Set(42)

	logger := zap.New(zap.UseDevMode(true))
	mock := &mockKubeClient{err: fmt.Errorf("load failed")}
	r := &AdditionalScrapeConfigReconciler{KubeClient: mock}
	config := &prometheusv1.AdditionalScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg-err-load", Namespace: "ns-err-load"},
	}

	_, err := r.loadTargets(context.Background(), logger, config)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	val := testutil.ToFloat64(scrapeJobsLoadedGauge.WithLabelValues("cfg-err-load", "ns-err-load"))
	if val != 42 {
		t.Errorf("scrape jobs loaded gauge = %v, want 42 (should not change on error)", val)
	}
}

func TestDeleteLabelValues_RemovesAllGaugeSeries(t *testing.T) {
	name, ns := "cfg-del", "ns-del"
	labels := map[string]string{"config_name": name, "config_namespace": ns}

	// Populate all three gauges
	discoveredJobsGauge.WithLabelValues(name, ns).Set(5)
	filteredJobsGauge.WithLabelValues(name, ns).Set(3)
	scrapeJobsLoadedGauge.WithLabelValues(name, ns).Set(8)

	if !gaugeLabelSetExists(discoveredJobsGauge, labels) {
		t.Fatal("discoveredJobsGauge should exist before deletion")
	}
	if !gaugeLabelSetExists(filteredJobsGauge, labels) {
		t.Fatal("filteredJobsGauge should exist before deletion")
	}
	if !gaugeLabelSetExists(scrapeJobsLoadedGauge, labels) {
		t.Fatal("scrapeJobsLoadedGauge should exist before deletion")
	}

	// Simulate the same cleanup the finalizer performs
	discoveredJobsGauge.DeleteLabelValues(name, ns)
	filteredJobsGauge.DeleteLabelValues(name, ns)
	scrapeJobsLoadedGauge.DeleteLabelValues(name, ns)

	if gaugeLabelSetExists(discoveredJobsGauge, labels) {
		t.Error("discoveredJobsGauge label set should be removed after DeleteLabelValues")
	}
	if gaugeLabelSetExists(filteredJobsGauge, labels) {
		t.Error("filteredJobsGauge label set should be removed after DeleteLabelValues")
	}
	if gaugeLabelSetExists(scrapeJobsLoadedGauge, labels) {
		t.Error("scrapeJobsLoadedGauge label set should be removed after DeleteLabelValues")
	}
}
