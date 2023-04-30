package v1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Namespace selector", func() {
	Context("When using any as the matcher", func() {
		sut := NamespaceSelector{
			Any:        true,
			MatchNames: nil,
		}
		It("Should match if the namespaces match", func() {
			Expect(sut.Matches("test", "test")).Should(BeTrue())
		})
		It("Should match if the namespaces do not match", func() {
			Expect(sut.Matches("test", "something-else")).Should(BeTrue())
		})
	})

	Context("When using nil matchers", func() {
		sut := NamespaceSelector{
			MatchNames: nil,
		}
		It("Should match if the namespaces match", func() {
			Expect(sut.Matches("test", "test")).Should(BeTrue())
		})
		It("Should not match if the namespaces do not match", func() {
			Expect(sut.Matches("test", "something-else")).Should(BeFalse())
		})
	})

	Context("When using name matchers with an empty slice", func() {
		sut := NamespaceSelector{
			MatchNames: []string{},
		}
		It("Should match if the namespaces match", func() {
			Expect(sut.Matches("test", "test")).Should(BeTrue())
		})
		It("Should not match if the namespaces do not match", func() {
			Expect(sut.Matches("test", "something-else")).Should(BeFalse())
		})
	})

	Context("When using name matchers with a non empty array", func() {
		sut := NamespaceSelector{
			MatchNames: []string{
				"valid",
			},
		}
		It("Should not match if the namespaces match without the namespace being in the list", func() {
			Expect(sut.Matches("test", "test")).Should(BeFalse())
		})
		It("Should match if the tested namespace is in the list", func() {
			Expect(sut.Matches("valid", "test")).Should(BeTrue())
		})
		It("Should not match if the tested namespace is not in the list", func() {
			Expect(sut.Matches("invalid", "test")).Should(BeFalse())
		})
	})
})
