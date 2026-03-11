package testutil

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"k8s.io/client-go/rest"
)

// SafeguardKubeconfig prevents tests from accidentally connecting to real clusters.
// It sets KUBECONFIG to an invalid path and unsets in-cluster config env vars.
func SafeguardKubeconfig() {
	if err := os.Setenv("KUBECONFIG", "/dev/null/nonexistent"); err != nil {
		panic(fmt.Sprintf("failed to set KUBECONFIG: %v", err))
	}
	if err := os.Unsetenv("KUBERNETES_SERVICE_HOST"); err != nil {
		panic(fmt.Sprintf("failed to unset KUBERNETES_SERVICE_HOST: %v", err))
	}
	if err := os.Unsetenv("KUBERNETES_SERVICE_PORT"); err != nil {
		panic(fmt.Sprintf("failed to unset KUBERNETES_SERVICE_PORT: %v", err))
	}
}

// ValidateTestConfig checks that a rest.Config points to a local address (envtest).
func ValidateTestConfig(cfg *rest.Config) error {
	if cfg == nil {
		return fmt.Errorf("rest.Config is nil")
	}

	host := cfg.Host
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}

	parsed, err := url.Parse(host)
	if err != nil {
		return fmt.Errorf("failed to parse config host %q: %w", cfg.Host, err)
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		hostname = cfg.Host
	}

	ip := net.ParseIP(hostname)
	if ip != nil {
		if ip.IsLoopback() {
			return nil
		}
		return fmt.Errorf("test config host %q is not a loopback address", cfg.Host)
	}

	if hostname == "localhost" {
		return nil
	}

	return fmt.Errorf("test config host %q is not a local address", cfg.Host)
}
