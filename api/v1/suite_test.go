package v1_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/szeber/kube-stager-prometheus-static-target/internal/testutil"
)

func TestV1(t *testing.T) {
	testutil.SafeguardKubeconfig()
	RegisterFailHandler(Fail)
	RunSpecs(t, "V1 Suite")
}
