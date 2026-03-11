package controller

import (
	"fmt"
	gomegaTypes "github.com/onsi/gomega/types"
	"github.com/szeber/kube-stager-prometheus-static-target/internal/prometheus"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

	deleteConfigAndSecret := func() {
		config := &prometheusv1.AdditionalScrapeConfig{}
		if err := k8sClient.Get(ctx, configLookupKey, config); err == nil {
			Expect(k8sClient.Delete(ctx, config)).Should(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, configLookupKey, config))
			}, timeout, interval).Should(BeTrue())
		}
		secret := &v1.Secret{}
		if err := k8sClient.Get(ctx, secretLookupKey, secret); err == nil {
			Expect(k8sClient.Delete(ctx, secret)).Should(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, secretLookupKey, secret))
			}, timeout, interval).Should(BeTrue())
		}
	}

	Context("When adding a matching Scrape Job", Ordered, func() {
		BeforeAll(func() {
			createNamespaces()
			createCommonJobs()
		})

		AfterEach(func() {
			deleteConfigAndSecret()
		})

		It("Should add the job to the status", func() {
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

	Context("When no ScrapeJobs match", Ordered, func() {
		AfterAll(func() {
			deleteConfigAndSecret()
		})

		It("Should have empty status and empty secret data", func() {
			config := prometheusv1.AdditionalScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ConfigName,
					Namespace: ConfigNamespace,
				},
				Spec: prometheusv1.AdditionalScrapeConfigSpec{
					SecretName:      SecretName,
					SecretNamespace: SecretNamespace,
					SecretKey:       SecretKey,
					ScrapeJobLabels: map[string]string{"target": "nonexistent"},
					ScrapeJobNamespaceSelector: prometheusv1.NamespaceSelector{
						Any: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, &config)).Should(Succeed())

			secret := &v1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, secretLookupKey, secret)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			yamlData, _ := yaml.Marshal([]prometheus.Job(nil))
			Expect(string(secret.Data[SecretKey])).To(MatchYAML(yamlData))

			createdConfig := &prometheusv1.AdditionalScrapeConfig{}
			Expect(k8sClient.Get(ctx, configLookupKey, createdConfig)).Should(Succeed())
			Expect(createdConfig.Status.DiscoveredScrapeJobs).To(BeNil())
		})
	})

	Context("When a ScrapeJob is deleted", Ordered, func() {
		var deletableJob *prometheusv1.ScrapeJob

		BeforeAll(func() {
			deletableJob = createJob("deletable-job", "test1", validJobLabels, prometheusv1.ScrapeJobSpec{
				JobName: "deletable",
				StaticConfigs: []prometheusv1.ScrapeJobStaticConfig{
					{Targets: []string{"http://deletable"}, Labels: map[string]string{"job": "deletable"}},
				},
			})
			createConfig()

			// Wait until the deletable job appears in status
			createdConfig := &prometheusv1.AdditionalScrapeConfig{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, configLookupKey, createdConfig); err != nil {
					return false
				}
				for _, j := range createdConfig.Status.DiscoveredScrapeJobs {
					if j == "test1/deletable-job" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

		AfterAll(func() {
			deleteConfigAndSecret()
		})

		It("Should remove the job from status and update secret after deletion", func() {
			Expect(k8sClient.Delete(ctx, deletableJob)).Should(Succeed())

			createdConfig := &prometheusv1.AdditionalScrapeConfig{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, configLookupKey, createdConfig); err != nil {
					return false
				}
				for _, j := range createdConfig.Status.DiscoveredScrapeJobs {
					if j == "test1/deletable-job" {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When a ScrapeJob is updated", Ordered, func() {
		BeforeAll(func() {
			createConfig()
		})

		AfterAll(func() {
			deleteConfigAndSecret()
		})

		It("Should reflect the updated job in the secret", func() {
			// Wait for initial reconciliation
			createdConfig := &prometheusv1.AdditionalScrapeConfig{}
			Eventually(func() ([]string, error) {
				if err := k8sClient.Get(ctx, configLookupKey, createdConfig); err != nil {
					return nil, err
				}
				return createdConfig.Status.DiscoveredScrapeJobs, nil
			}, timeout, interval).Should(ContainElement("test1/valid-1"))

			// Update the job
			job := &prometheusv1.ScrapeJob{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "test1", Name: "valid-1"}, job)).Should(Succeed())
			job.Spec.StaticConfigs[0].Targets = []string{"http://updated-target"}
			Expect(k8sClient.Update(ctx, job)).Should(Succeed())

			// Verify the secret reflects the update
			secret := &v1.Secret{}
			Eventually(func() string {
				if err := k8sClient.Get(ctx, secretLookupKey, secret); err != nil {
					return ""
				}
				return string(secret.Data[SecretKey])
			}, timeout, interval).Should(ContainSubstring("http://updated-target"))
		})
	})

	Context("When the secret is externally modified", Ordered, func() {
		BeforeAll(func() {
			createConfig()
		})

		AfterAll(func() {
			deleteConfigAndSecret()
		})

		It("Should restore the secret to the correct state", func() {
			// Wait for initial secret creation
			secret := &v1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, secretLookupKey, secret)
				return err == nil && len(secret.Data[SecretKey]) > 0
			}, timeout, interval).Should(BeTrue())

			// Tamper with the secret
			secret.Data[SecretKey] = []byte("tampered")
			Expect(k8sClient.Update(ctx, secret)).Should(Succeed())

			// Verify the controller restores it
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, secretLookupKey, secret); err != nil {
					return false
				}
				return string(secret.Data[SecretKey]) != "tampered" && len(secret.Data[SecretKey]) > 0
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When a ScrapeJob's labels change to no longer match", Ordered, func() {
		var mutableJob *prometheusv1.ScrapeJob

		BeforeAll(func() {
			mutableJob = createJob("mutable-job", "test1", validJobLabels, prometheusv1.ScrapeJobSpec{
				JobName: "mutable",
				StaticConfigs: []prometheusv1.ScrapeJobStaticConfig{
					{Targets: []string{"http://mutable"}, Labels: map[string]string{"job": "mutable"}},
				},
			})
			createConfig()

			// Wait until the mutable job appears in status
			createdConfig := &prometheusv1.AdditionalScrapeConfig{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, configLookupKey, createdConfig); err != nil {
					return false
				}
				for _, j := range createdConfig.Status.DiscoveredScrapeJobs {
					if j == "test1/mutable-job" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

		AfterAll(func() {
			deleteConfigAndSecret()
			if mutableJob != nil {
				_ = k8sClient.Delete(ctx, mutableJob)
			}
		})

		It("Should remove the job from status after labels change", func() {
			// Change labels so it no longer matches
			job := &prometheusv1.ScrapeJob{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "test1", Name: "mutable-job"}, job)).Should(Succeed())
			job.Labels = map[string]string{"target": "no-match"}
			Expect(k8sClient.Update(ctx, job)).Should(Succeed())

			createdConfig := &prometheusv1.AdditionalScrapeConfig{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, configLookupKey, createdConfig); err != nil {
					return false
				}
				for _, j := range createdConfig.Status.DiscoveredScrapeJobs {
					if j == "test1/mutable-job" {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When NamespaceSelector.Any is true", Ordered, func() {
		AfterAll(func() {
			deleteConfigAndSecret()
		})

		It("Should discover jobs in all namespaces", func() {
			config := prometheusv1.AdditionalScrapeConfig{
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
						Any: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, &config)).Should(Succeed())

			createdConfig := &prometheusv1.AdditionalScrapeConfig{}
			Eventually(func() ([]string, error) {
				if err := k8sClient.Get(ctx, configLookupKey, createdConfig); err != nil {
					return nil, err
				}
				return createdConfig.Status.DiscoveredScrapeJobs, nil
			}, timeout, interval).Should(ContainElement("test3/different-namespace"))
		})
	})

	Context("When NamespaceSelector has no MatchNames", Ordered, func() {
		AfterAll(func() {
			deleteConfigAndSecret()
		})

		It("Should only discover jobs in the config's own namespace", func() {
			// Create a job in default namespace with valid labels
			ownNsJob := createJob("own-ns-job", ConfigNamespace, validJobLabels, prometheusv1.ScrapeJobSpec{
				JobName: "own-ns",
				StaticConfigs: []prometheusv1.ScrapeJobStaticConfig{
					{Targets: []string{"http://own-ns"}, Labels: map[string]string{"job": "own-ns"}},
				},
			})

			config := prometheusv1.AdditionalScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ConfigName,
					Namespace: ConfigNamespace,
				},
				Spec: prometheusv1.AdditionalScrapeConfigSpec{
					SecretName:                 SecretName,
					SecretNamespace:            SecretNamespace,
					SecretKey:                  SecretKey,
					ScrapeJobLabels:            validJobLabels,
					ScrapeJobNamespaceSelector: prometheusv1.NamespaceSelector{},
				},
			}
			Expect(k8sClient.Create(ctx, &config)).Should(Succeed())

			createdConfig := &prometheusv1.AdditionalScrapeConfig{}
			Eventually(func() ([]string, error) {
				if err := k8sClient.Get(ctx, configLookupKey, createdConfig); err != nil {
					return nil, err
				}
				return createdConfig.Status.DiscoveredScrapeJobs, nil
			}, timeout, interval).Should(And(
				ContainElement(fmt.Sprintf("%s/own-ns-job", ConfigNamespace)),
				Not(ContainElement("test1/valid-1")),
			))

			// Cleanup the job we created
			Expect(k8sClient.Delete(ctx, ownNsJob)).Should(Succeed())
		})
	})

	Context("Finalizer lifecycle", Ordered, func() {
		AfterAll(func() {
			deleteConfigAndSecret()
		})

		It("Should add a finalizer after creation and clean up gauges on deletion", func() {
			createConfig()

			createdConfig := &prometheusv1.AdditionalScrapeConfig{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, configLookupKey, createdConfig); err != nil {
					return false
				}
				return controllerutil.ContainsFinalizer(createdConfig, metricsFinalizerName)
			}, timeout, interval).Should(BeTrue())

			gaugeLabels := map[string]string{"config_name": ConfigName, "config_namespace": ConfigNamespace}

			// Wait for gauges to be populated by reconciliation
			Eventually(func() bool {
				return gaugeLabelSetExists(discoveredJobsGauge, gaugeLabels)
			}, timeout, interval).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, createdConfig)).Should(Succeed())

			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, configLookupKey, createdConfig))
			}, timeout, interval).Should(BeTrue())

			// Finalizer should have removed gauge label sets
			Expect(gaugeLabelSetExists(discoveredJobsGauge, gaugeLabels)).To(BeFalse())
			Expect(gaugeLabelSetExists(filteredJobsGauge, gaugeLabels)).To(BeFalse())
			Expect(gaugeLabelSetExists(scrapeJobsLoadedGauge, gaugeLabels)).To(BeFalse())
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

	remaining := make(map[string][]byte, len(actualMap)-1)
	for k, v := range actualMap {
		if k != r.secretKey {
			remaining[k] = v
		}
	}

	return r.equalMatcher.Match(remaining)
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
