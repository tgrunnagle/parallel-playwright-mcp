package tools

import (
	"slices"
	"testing"
)

// registeredTools returns the tools the server registers. Handlers are built
// without dependencies; the tests only read the definitions.
func registeredTools() []string {
	var names []string
	for _, st := range Registry(nil, nil, nil) {
		names = append(names, st.Tool.Name)
	}
	return names
}

// TestAllToolNamesMatchesRegistry checks that AllToolNames, which config
// validation uses, names exactly the tools the server registers.
func TestAllToolNamesMatchesRegistry(t *testing.T) {
	registered := registeredTools()
	if len(registered) != len(AllToolNames) {
		t.Errorf("Registry has %d tools, AllToolNames has %d", len(registered), len(AllToolNames))
	}
	seen := make(map[string]bool, len(registered))
	for _, name := range registered {
		if seen[name] {
			t.Errorf("tool %q is registered twice", name)
		}
		seen[name] = true
		if !slices.Contains(AllToolNames, name) {
			t.Errorf("registered tool %q is missing from AllToolNames", name)
		}
	}
	for _, name := range AllToolNames {
		if !seen[name] {
			t.Errorf("AllToolNames lists %q, which is not registered", name)
		}
	}
}
