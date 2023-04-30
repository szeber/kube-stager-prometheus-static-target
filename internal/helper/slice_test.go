package helper

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("StringInStringSlice", func() {
	Context("When checking if a slice contains a string", func() {
		It("Should return true if the slice contains the string", func() {
			Expect(StringInStringSlice("test2", []string{"test1", "test2", "test3"})).Should(BeTrue())
		})
		It("Should return false if the slice does not contain the string", func() {
			Expect(StringInStringSlice("test", []string{"test1", "test2", "test3"})).Should(BeFalse())
		})
	})
})
