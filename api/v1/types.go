package v1

import "github.com/szeber/kube-stager-prometheus-static-target/internal/helper"

type NamespaceSelector struct {
	// Boolean describing whether all namespaces are selected in contrast to a
	// list restricting them.
	Any bool `json:"any,omitempty"`
	// List of namespace names to select from.
	MatchNames []string `json:"matchNames,omitempty"`
}

func (r *NamespaceSelector) Matches(namespace string, ownNamespace string) bool {
	if r.Any {
		return true
	}

	if len(r.MatchNames) == 0 {
		return namespace == ownNamespace
	}

	res := helper.StringInStringSlice(namespace, r.MatchNames)

	return res
}
