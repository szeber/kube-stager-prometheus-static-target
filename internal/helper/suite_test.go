package helper_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/szeber/kube-stager-prometheus-static-target/internal/testutil"
)

func TestHelper(t *testing.T) {
	testutil.SafeguardKubeconfig()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Helper Suite")
}
