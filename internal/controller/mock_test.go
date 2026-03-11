package controller

import (
	"context"

	prometheusv1 "github.com/szeber/kube-stager-prometheus-static-target/api/v1"
	"github.com/szeber/kube-stager-prometheus-static-target/internal/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Compile-time check that mockKubeClient satisfies the interface.
var _ kubernetes.ClientInterface = (*mockKubeClient)(nil)

type mockKubeClient struct {
	// err is the default error returned by all methods except GetSecret
	// (which uses secretErr when non-nil) and CreateOrUpdateSecret (which
	// uses createUpdateFn when non-nil).
	err error

	secret       *corev1.Secret
	secretExists bool
	// secretErr, when non-nil, overrides err for GetSecret only.
	secretErr error

	createUpdateFn func(ctx context.Context, secretExists bool, secret *corev1.Secret) error

	scrapeJobs *prometheusv1.ScrapeJobList

	configs    *prometheusv1.AdditionalScrapeConfigList
	allConfigs *prometheusv1.AdditionalScrapeConfigList
}

func (m *mockKubeClient) GetAdditionalScrapeConfig(_ context.Context, _ string, _ string) (*prometheusv1.AdditionalScrapeConfig, error) {
	return nil, m.err
}

func (m *mockKubeClient) LoadScrapeJobs(_ context.Context, _ *prometheusv1.AdditionalScrapeConfig) (*prometheusv1.ScrapeJobList, error) {
	return m.scrapeJobs, m.err
}

func (m *mockKubeClient) GetSecret(_ context.Context, _ *prometheusv1.AdditionalScrapeConfig) (*corev1.Secret, bool, error) {
	if m.secretErr != nil {
		return nil, false, m.secretErr
	}
	return m.secret, m.secretExists, m.err
}

func (m *mockKubeClient) CreateOrUpdateSecret(ctx context.Context, secretExists bool, secret *corev1.Secret) error {
	if m.createUpdateFn != nil {
		return m.createUpdateFn(ctx, secretExists, secret)
	}
	return m.err
}

func (m *mockKubeClient) FindAdditionalScrapeConfigsForSecret(_ context.Context, _ client.Object) (*prometheusv1.AdditionalScrapeConfigList, error) {
	return m.configs, m.err
}

func (m *mockKubeClient) GetAllAdditionalScrapeConfigs(_ context.Context) (*prometheusv1.AdditionalScrapeConfigList, error) {
	return m.allConfigs, m.err
}
