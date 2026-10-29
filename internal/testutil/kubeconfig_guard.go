package testutil

import (
	"errors"
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

func RequireLocalAPIServer(cfg *rest.Config) error {
	if cfg == nil {
		return errors.New("rest.Config is nil")
	}

	host := cfg.Host
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}

	apiServerURL, err := url.Parse(host)
	if err != nil {
		return fmt.Errorf("failed to parse config host %q: %w", cfg.Host, err)
	}

	hostname := apiServerURL.Hostname()
	ip := net.ParseIP(hostname)
	if hostname == "localhost" || (ip != nil && ip.IsLoopback()) {
		return nil
	}

	return fmt.Errorf("test API server %q is not on a loopback address", cfg.Host)
}
