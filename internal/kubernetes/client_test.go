package kubernetes

import (
	"context"
	"testing"

	prometheusv1 "github.com/szeber/kube-stager-prometheus-static-target/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = prometheusv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	return s
}

func newFakeClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(objs...).Build()
}

func TestGetAdditionalScrapeConfig_Exists(t *testing.T) {
	config := &prometheusv1.AdditionalScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: prometheusv1.AdditionalScrapeConfigSpec{
			SecretName: "my-secret",
		},
	}
	c := NewClient(newFakeClient(config))
	got, err := c.GetAdditionalScrapeConfig(context.Background(), "default", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Spec.SecretName != "my-secret" {
		t.Errorf("SecretName = %q, want %q", got.Spec.SecretName, "my-secret")
	}
}

func TestGetAdditionalScrapeConfig_NotFound(t *testing.T) {
	c := NewClient(newFakeClient())
	_, err := c.GetAdditionalScrapeConfig(context.Background(), "default", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent config")
	}
}

func TestLoadScrapeJobs_MatchingLabels(t *testing.T) {
	job1 := &prometheusv1.ScrapeJob{
		ObjectMeta: metav1.ObjectMeta{Name: "job1", Namespace: "ns1", Labels: map[string]string{"app": "test"}},
		Spec:       prometheusv1.ScrapeJobSpec{JobName: "j1"},
	}
	job2 := &prometheusv1.ScrapeJob{
		ObjectMeta: metav1.ObjectMeta{Name: "job2", Namespace: "ns1", Labels: map[string]string{"app": "other"}},
		Spec:       prometheusv1.ScrapeJobSpec{JobName: "j2"},
	}
	c := NewClient(newFakeClient(job1, job2))

	config := &prometheusv1.AdditionalScrapeConfig{
		Spec: prometheusv1.AdditionalScrapeConfigSpec{
			ScrapeJobLabels: map[string]string{"app": "test"},
		},
	}
	list, err := c.LoadScrapeJobs(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(list.Items))
	}
	if list.Items[0].Spec.JobName != "j1" {
		t.Errorf("JobName = %q, want %q", list.Items[0].Spec.JobName, "j1")
	}
}

func TestLoadScrapeJobs_NoMatch(t *testing.T) {
	job := &prometheusv1.ScrapeJob{
		ObjectMeta: metav1.ObjectMeta{Name: "job1", Namespace: "ns1", Labels: map[string]string{"app": "other"}},
	}
	c := NewClient(newFakeClient(job))

	config := &prometheusv1.AdditionalScrapeConfig{
		Spec: prometheusv1.AdditionalScrapeConfigSpec{
			ScrapeJobLabels: map[string]string{"app": "test"},
		},
	}
	list, err := c.LoadScrapeJobs(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list.Items) != 0 {
		t.Errorf("got %d items, want 0", len(list.Items))
	}
}

func TestGetSecret_Exists(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
		Data:       map[string][]byte{"key": []byte("value")},
		Type:       corev1.SecretTypeOpaque,
	}
	c := NewClient(newFakeClient(secret))

	config := &prometheusv1.AdditionalScrapeConfig{
		Spec: prometheusv1.AdditionalScrapeConfigSpec{
			SecretName:      "my-secret",
			SecretNamespace: "default",
		},
	}
	got, exists, err := c.GetSecret(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected secret to exist")
	}
	if string(got.Data["key"]) != "value" {
		t.Errorf("Data[key] = %q, want %q", got.Data["key"], "value")
	}
}

func TestGetSecret_NotFound(t *testing.T) {
	c := NewClient(newFakeClient())

	config := &prometheusv1.AdditionalScrapeConfig{
		Spec: prometheusv1.AdditionalScrapeConfigSpec{
			SecretName:      "missing",
			SecretNamespace: "default",
		},
	}
	got, exists, err := c.GetSecret(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected secret to not exist")
	}
	if got.Name != "missing" {
		t.Errorf("Name = %q, want %q", got.Name, "missing")
	}
	if got.Namespace != "default" {
		t.Errorf("Namespace = %q, want %q", got.Namespace, "default")
	}
	if got.Type != corev1.SecretTypeOpaque {
		t.Errorf("Type = %q, want %q", got.Type, corev1.SecretTypeOpaque)
	}
}

func TestCreateOrUpdateSecret_Create(t *testing.T) {
	c := NewClient(newFakeClient())

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "new-secret", Namespace: "default"},
		Data:       map[string][]byte{"key": []byte("value")},
		Type:       corev1.SecretTypeOpaque,
	}
	err := c.CreateOrUpdateSecret(context.Background(), false, secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it was created
	got := &corev1.Secret{}
	err = c.parentClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "new-secret"}, got)
	if err != nil {
		t.Fatalf("secret not found after create: %v", err)
	}
}

func TestCreateOrUpdateSecret_Update(t *testing.T) {
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "existing", Namespace: "default"},
		Data:       map[string][]byte{"key": []byte("old")},
		Type:       corev1.SecretTypeOpaque,
	}
	c := NewClient(newFakeClient(existing))

	// Re-fetch to get resource version
	fetched := &corev1.Secret{}
	_ = c.parentClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "existing"}, fetched)
	fetched.Data["key"] = []byte("new")

	err := c.CreateOrUpdateSecret(context.Background(), true, fetched)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := &corev1.Secret{}
	_ = c.parentClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "existing"}, got)
	if string(got.Data["key"]) != "new" {
		t.Errorf("Data[key] = %q, want %q", got.Data["key"], "new")
	}
}

func TestGetAllAdditionalScrapeConfigs_Populated(t *testing.T) {
	c1 := &prometheusv1.AdditionalScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
	}
	c2 := &prometheusv1.AdditionalScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "c2", Namespace: "other"},
	}
	c := NewClient(newFakeClient(c1, c2))
	list, err := c.GetAllAdditionalScrapeConfigs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list.Items) != 2 {
		t.Errorf("got %d items, want 2", len(list.Items))
	}
}

func TestGetAllAdditionalScrapeConfigs_Empty(t *testing.T) {
	c := NewClient(newFakeClient())
	list, err := c.GetAllAdditionalScrapeConfigs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list.Items) != 0 {
		t.Errorf("got %d items, want 0", len(list.Items))
	}
}
