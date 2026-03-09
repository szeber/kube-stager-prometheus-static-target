package kubernetes

import (
	"context"
	prometheusv1 "github.com/szeber/kube-stager-prometheus-static-target/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Client struct {
	parentClient client.Client
}

func NewClient(parentClient client.Client) *Client {
	return &Client{
		parentClient: parentClient,
	}
}

func (r *Client) GetAdditionalScrapeConfig(ctx context.Context, namespace string, name string) (*prometheusv1.AdditionalScrapeConfig, error) {
	config := &prometheusv1.AdditionalScrapeConfig{}
	err := r.parentClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, config)

	return config, err
}

func (r *Client) LoadScrapeJobs(ctx context.Context, config *prometheusv1.AdditionalScrapeConfig) (*prometheusv1.ScrapeJobList, error) {
	scrapeJobList := &prometheusv1.ScrapeJobList{}
	err := r.parentClient.List(ctx, scrapeJobList, client.MatchingLabels(config.Spec.ScrapeJobLabels))

	return scrapeJobList, err
}

func (r *Client) GetSecret(ctx context.Context, config *prometheusv1.AdditionalScrapeConfig) (*corev1.Secret, bool, error) {
	secret := &corev1.Secret{}
	secretExists := true

	err := r.parentClient.Get(
		ctx,
		client.ObjectKey{Namespace: config.Spec.SecretNamespace, Name: config.Spec.SecretName},
		secret,
	)

	if nil != err {
		statusError, ok := err.(*errors.StatusError)
		if !ok || statusError.Status().Code != 404 {
			return nil, false, err
		}
		secretExists = false
		secret.Namespace = config.Spec.SecretNamespace
		secret.Name = config.Spec.SecretName
		secret.Type = corev1.SecretTypeOpaque
	}

	return secret, secretExists, nil
}

func (r *Client) CreateOrUpdateSecret(ctx context.Context, secretExists bool, secret *corev1.Secret) error {
	if secretExists {
		return r.parentClient.Update(ctx, secret)
	}

	return r.parentClient.Create(ctx, secret)
}

func (r *Client) FindAdditionalScrapeConfigsForSecret(ctx context.Context, secret client.Object) (*prometheusv1.AdditionalScrapeConfigList, error) {
	configList := &prometheusv1.AdditionalScrapeConfigList{}
	listOpts := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(".spec.secretName", secret.GetName()),
	}
	err := r.parentClient.List(ctx, configList, listOpts)

	return configList, err
}

func (r *Client) GetAllAdditionalScrapeConfigs(ctx context.Context) (*prometheusv1.AdditionalScrapeConfigList, error) {
	allConfigs := &prometheusv1.AdditionalScrapeConfigList{}
	err := r.parentClient.List(ctx, allConfigs)

	return allConfigs, err
}
