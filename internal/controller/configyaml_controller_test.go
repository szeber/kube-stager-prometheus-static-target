package controller

import (
	"context"
	"fmt"
	gomegaTypes "github.com/onsi/gomega/types"
	"github.com/szeber/kube-stager-prometheus-static-target/internal/prometheus"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	prometheusv1 "github.com/szeber/kube-stager-prometheus-static-target/api/v1"
)

const (
	ConfigName      = "test-config"
	ConfigNamespace = "default"
	SecretName      = "test-secret"
	SecretNamespace = "default"
	SecretKey       = "test"

	timeout  = time.Second * 10
	interval = time.Millisecond * 250
)

var _ = Describe("Additional Scrape Config controller", func() {
	validJobLabels := map[string]string{"target": "test"}
	invalidJobLabels := map[string]string{"target": "invalid"}
	matchedNamespaces := []string{"test1", "test2"}

	getConfig := func() prometheusv1.AdditionalScrapeConfig {
		return prometheusv1.AdditionalScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ConfigName,
				Namespace: ConfigNamespace,
			},
			Spec: prometheusv1.AdditionalScrapeConfigSpec{
				SecretName:      SecretName,
				SecretNamespace: SecretNamespace,
				SecretKey:       SecretKey,
				ScrapeJobLabels: validJobLabels,
				ScrapeJobNamespaceSelector: prometheusv1.NamespaceSelector{
					MatchNames: matchedNamespaces,
				},
			},
		}
	}

	configLookupKey := types.NamespacedName{Name: ConfigName, Namespace: ConfigNamespace}
	secretLookupKey := types.NamespacedName{Name: SecretName, Namespace: SecretNamespace}

	ctx := context.Background()

	var matchingJob1 *prometheusv1.ScrapeJob
	var matchingJob2 *prometheusv1.ScrapeJob

	createNamespace := func(name string) {
		namespace := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
		Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())
	}

	createNamespaces := func() {
		createNamespace("test1")
		createNamespace("test2")
		createNamespace("test3")
	}

	createJob := func(name string, namespace string, labels map[string]string, spec prometheusv1.ScrapeJobSpec) *prometheusv1.ScrapeJob {
		job := &prometheusv1.ScrapeJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    labels,
			},
			Spec: spec,
		}
		Expect(k8sClient.Create(ctx, job)).Should(Succeed())

		createdJob := &prometheusv1.ScrapeJob{}

		Eventually(func() bool {
			return nil == k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, createdJob)
		}, timeout, interval).Should(BeTrue())

		return createdJob
	}

	createCommonJobs := func() {
		matchingJob1 = createJob("valid-1", "test1", validJobLabels, prometheusv1.ScrapeJobSpec{
			JobName: "test1",
			StaticConfigs: []prometheusv1.ScrapeJobStaticConfig{
				{
					Targets: []string{"http://test1"},
					Labels:  map[string]string{"job": "test1"},
				},
			},
		})
		matchingJob2 = createJob("valid-2", "test2", validJobLabels, prometheusv1.ScrapeJobSpec{
			JobName: "test2",
			StaticConfigs: []prometheusv1.ScrapeJobStaticConfig{
				{
					Targets: []string{"http://test2"},
					Labels:  map[string]string{"job": "test2"},
				},
			},
		})
		_ = createJob("different-namespace", "test3", validJobLabels, prometheusv1.ScrapeJobSpec{
			JobName: "different-namespace",
			StaticConfigs: []prometheusv1.ScrapeJobStaticConfig{
				{
					Targets: []string{"http://different-namespace"},
					Labels:  map[string]string{"job": "different-namespace"},
				},
			},
		})
		_ = createJob("invalid", "test1", invalidJobLabels, prometheusv1.ScrapeJobSpec{
			JobName: "invalid",
			StaticConfigs: []prometheusv1.ScrapeJobStaticConfig{
				{
					Targets: []string{"http://invalid"},
					Labels:  map[string]string{"job": "invalid"},
				},
			},
		})
	}
	createConfig := func() *prometheusv1.AdditionalScrapeConfig {
		config := getConfig()
		Expect(k8sClient.Create(ctx, &config)).Should(Succeed())
		createdConfig := &prometheusv1.AdditionalScrapeConfig{}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, configLookupKey, createdConfig)
			return nil == err
		}, timeout, interval).Should(BeTrue())

		return createdConfig
	}

	createSecret := func(data map[string][]byte) {
		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      SecretName,
				Namespace: SecretNamespace,
			},
			Data: data,
			Type: "opaque",
		}
		Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

		createdSecret := &v1.Secret{}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, secretLookupKey, createdSecret)
			return nil == err
		}, timeout, interval).Should(BeTrue())
	}

	getPrometheusData := func() []prometheus.Job {
		return []prometheus.Job{
			{
				JobName: matchingJob1.Spec.JobName,
				StaticConfigs: []prometheus.StaticConfig{
					{
						Targets: matchingJob1.Spec.StaticConfigs[0].Targets,
						Labels:  matchingJob1.Spec.StaticConfigs[0].Labels,
					},
				},
			},
			{
				JobName: matchingJob2.Spec.JobName,
				StaticConfigs: []prometheus.StaticConfig{
					{
						Targets: matchingJob2.Spec.StaticConfigs[0].Targets,
						Labels:  matchingJob2.Spec.StaticConfigs[0].Labels,
					},
				},
			},
		}
	}

	Context("When adding a matching Scrape Job", func() {
		It("Should add the job to the status", func() {
			createNamespaces()
			createCommonJobs()
			createdConfig := createConfig()

			By("By checking the status's discovered jobs")
			Eventually(func() ([]string, error) {
				if err := k8sClient.Get(ctx, configLookupKey, createdConfig); nil != err {
					return nil, err
				}

				return createdConfig.Status.DiscoveredScrapeJobs, nil
			}).Should(Equal([]string{"test1/valid-1", "test2/valid-2"}))
		})
		It("Should update the existing secret overwriting the key", func() {
			createNamespaces()
			createCommonJobs()
			createSecret(map[string][]byte{"otherKey": []byte("test"), SecretKey: []byte("test2")})
			createConfig()

			secret := &v1.Secret{}

			Eventually(func() (map[string][]byte, error) {
				if err := k8sClient.Get(ctx, secretLookupKey, secret); nil != err {
					return nil, err
				}

				return secret.Data, nil
			}).Should(matchSecretData(SecretKey, getPrometheusData(), map[string][]byte{"otherKey": []byte("test")}))
		})
		It("Should update the existing secret", func() {
			createNamespaces()
			createCommonJobs()
			createSecret(nil)
			createConfig()

			secret := &v1.Secret{}

			Eventually(func() (map[string][]byte, error) {
				if err := k8sClient.Get(ctx, secretLookupKey, secret); nil != err {
					return nil, err
				}

				return secret.Data, nil
			}).Should(matchSecretData(SecretKey, getPrometheusData(), map[string][]byte{}))
		})
		It("Should create the secret if it does not exist", func() {
			createNamespaces()
			createCommonJobs()
			createConfig()

			secret := &v1.Secret{}

			Eventually(func() (map[string][]byte, error) {
				if err := k8sClient.Get(ctx, secretLookupKey, secret); nil != err {
					return nil, err
				}

				return secret.Data, nil
			}).Should(matchSecretData(SecretKey, getPrometheusData(), map[string][]byte{}))
		})
	})
})

func matchSecretData(secretKey string, prometheusData []prometheus.Job, remainingSecretData map[string][]byte) gomegaTypes.GomegaMatcher {
	yamlBytes, _ := yaml.Marshal(prometheusData)
	return &matchSecretDataMatcher{
		secretKey:    secretKey,
		yamlMatcher:  MatchYAML(yamlBytes),
		equalMatcher: Equal(remainingSecretData),
		trueMatcher:  BeTrue(),
	}

}

type matchSecretDataMatcher struct {
	secretKey    string
	yamlMatcher  gomegaTypes.GomegaMatcher
	equalMatcher gomegaTypes.GomegaMatcher
	trueMatcher  gomegaTypes.GomegaMatcher
}

func (r *matchSecretDataMatcher) Match(actual interface{}) (success bool, err error) {
	actualMap := actual.(map[string][]byte)

	prometheusData, exists := actualMap[r.secretKey]
	if success, err := r.trueMatcher.Match(exists); nil != err {
		return success, err
	} else if !success {
		return success, nil
	}

	if success, err := r.yamlMatcher.Match(prometheusData); nil != err {
		return success, err
	} else if !success {
		return success, nil
	}

	delete(actualMap, r.secretKey)

	return r.equalMatcher.Match(actualMap)
}

func (r *matchSecretDataMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Failed to assert that %+v is a valid secret data", r.getActualAsStringMap(actual))
}

func (r *matchSecretDataMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Failed to assert that %+v is not a valid secret data", r.getActualAsStringMap(actual))
}

func (r *matchSecretDataMatcher) getActualAsStringMap(actual interface{}) map[string]string {
	actualMap := actual.(map[string][]byte)
	stringData := make(map[string]string)

	for k, v := range actualMap {
		stringData[k] = "\n" + string(v) + "\n"
	}

	return stringData
}
