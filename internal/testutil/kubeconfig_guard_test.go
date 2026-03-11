package testutil

import (
	"os"
	"testing"

	"k8s.io/client-go/rest"
)

func TestSafeguardKubeconfig(t *testing.T) {
	originalKubeconfig := os.Getenv("KUBECONFIG")
	originalHost := os.Getenv("KUBERNETES_SERVICE_HOST")
	originalPort := os.Getenv("KUBERNETES_SERVICE_PORT")
	t.Cleanup(func() {
		t.Helper()
		if err := os.Setenv("KUBECONFIG", originalKubeconfig); err != nil {
			t.Fatalf("failed to restore KUBECONFIG: %v", err)
		}
		if originalHost != "" {
			if err := os.Setenv("KUBERNETES_SERVICE_HOST", originalHost); err != nil {
				t.Fatalf("failed to restore KUBERNETES_SERVICE_HOST: %v", err)
			}
		}
		if originalPort != "" {
			if err := os.Setenv("KUBERNETES_SERVICE_PORT", originalPort); err != nil {
				t.Fatalf("failed to restore KUBERNETES_SERVICE_PORT: %v", err)
			}
		}
	})

	if err := os.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1"); err != nil {
		t.Fatalf("failed to set KUBERNETES_SERVICE_HOST: %v", err)
	}
	if err := os.Setenv("KUBERNETES_SERVICE_PORT", "443"); err != nil {
		t.Fatalf("failed to set KUBERNETES_SERVICE_PORT: %v", err)
	}

	SafeguardKubeconfig()

	if got := os.Getenv("KUBECONFIG"); got != "/dev/null/nonexistent" {
		t.Errorf("KUBECONFIG = %q, want /dev/null/nonexistent", got)
	}
	if _, ok := os.LookupEnv("KUBERNETES_SERVICE_HOST"); ok {
		t.Error("KUBERNETES_SERVICE_HOST should be unset")
	}
	if _, ok := os.LookupEnv("KUBERNETES_SERVICE_PORT"); ok {
		t.Error("KUBERNETES_SERVICE_PORT should be unset")
	}
}

func TestValidateTestConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *rest.Config
		wantErr bool
	}{
		{"nil config", nil, true},
		{"localhost", &rest.Config{Host: "https://localhost:6443"}, false},
		{"127.0.0.1", &rest.Config{Host: "https://127.0.0.1:6443"}, false},
		{"ipv6 loopback", &rest.Config{Host: "https://[::1]:6443"}, false},
		{"remote host", &rest.Config{Host: "https://10.0.0.1:6443"}, true},
		{"remote hostname", &rest.Config{Host: "https://my-cluster.example.com:6443"}, true},
		{"bare 127.0.0.1", &rest.Config{Host: "127.0.0.1:6443"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTestConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTestConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
