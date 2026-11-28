package tools

import (
	"regexp"
	"slices"
	"strings"
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

// TestToolAnnotations checks every tool's four annotation hints against the
// annotation table in docs/design/mvp_design.md section 5. mcp.NewTool
// defaults to destructive and open-world, so every hint, false ones included,
// must be set explicitly.
func TestToolAnnotations(t *testing.T) {
	type hints struct{ readOnly, destructive, idempotent, openWorld bool }
	var (
		readOnlyLocal = hints{readOnly: true}
		readOnlyPage  = hints{readOnly: true, openWorld: true}
		pageChange    = hints{openWorld: true}
		pageAction    = hints{destructive: true, openWorld: true}
	)
	want := map[string]hints{
		"get_console_logs":          readOnlyLocal,
		"get_network_logs":          readOnlyLocal,
		"session_list":              readOnlyLocal,
		"screenshot":                readOnlyPage,
		"extract_text":              readOnlyPage,
		"get_html":                  readOnlyPage,
		"query_selector":            readOnlyPage,
		"get_accessibility_tree":    readOnlyPage,
		"session_create":            {},
		"navigate":                  pageChange,
		"go_back":                   pageChange,
		"go_forward":                pageChange,
		"reload":                    pageChange,
		"navigate_and_extract_text": pageChange,
		"get_links":                 pageChange,
		"hover":                     pageChange,
		"click":                     pageAction,
		"type":                      pageAction,
		"fill":                      pageAction,
		"select_option":             pageAction,
		"press_key":                 pageAction,
		"session_close":             {destructive: true, idempotent: true},
		"evaluate":                  pageAction,
	}

	tools := Registry(nil, nil, nil)
	if len(tools) != len(want) {
		t.Errorf("Registry has %d tools, table has %d", len(tools), len(want))
	}
	for _, st := range tools {
		name := st.Tool.Name
		t.Run(name, func(t *testing.T) {
			w, ok := want[name]
			if !ok {
				t.Fatalf("tool %q has no row in the annotation table", name)
			}
			a := st.Tool.Annotations
			for _, h := range []struct {
				hint string
				got  *bool
				want bool
			}{
				{"readOnlyHint", a.ReadOnlyHint, w.readOnly},
				{"destructiveHint", a.DestructiveHint, w.destructive},
				{"idempotentHint", a.IdempotentHint, w.idempotent},
				{"openWorldHint", a.OpenWorldHint, w.openWorld},
			} {
				if h.got == nil {
					t.Errorf("%s is unset, want %v", h.hint, h.want)
				} else if *h.got != h.want {
					t.Errorf("%s = %v, want %v", h.hint, *h.got, h.want)
				}
			}
		})
	}
}

// sentenceBreak matches the end of one sentence and the start of the next,
// whatever the next one starts with: a capital, a tool name, or a backtick.
var sentenceBreak = regexp.MustCompile(`[.!?]\s+\S`)

// abbreviation matches the abbreviations that end in a period without ending
// a sentence.
var abbreviation = regexp.MustCompile(`\b(e\.g|i\.e|etc|vs)\.`)

// countSentences returns the number of sentences in s.
func countSentences(s string) int {
	return len(sentenceBreak.FindAllStringIndex(abbreviation.ReplaceAllString(s, ""), -1)) + 1
}

func TestCountSentences(t *testing.T) {
	for _, tc := range []struct {
		text string
		want int
	}{
		{"One sentence.", 1},
		{"One. Two.", 2},
		{"Use it first. navigate_and_extract_text does both.", 2},
		{"Returns refs. `ref` names one element.", 2},
		{"Done? Yes! Next.", 3},
		{"Press a key, e.g. Enter, or i.e. Tab.", 1},
	} {
		if got := countSentences(tc.text); got != tc.want {
			t.Errorf("countSentences(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
}

// TestToolDescriptions checks that every description is two to four sentences
// and that every selector parameter documents the shared selector syntax.
func TestToolDescriptions(t *testing.T) {
	for _, st := range Registry(nil, nil, nil) {
		t.Run(st.Tool.Name, func(t *testing.T) {
			desc := st.Tool.Description
			sentences := countSentences(desc)
			if sentences < 2 || sentences > 4 || strings.TrimRight(desc, ".!?") == desc {
				t.Errorf("description is %d sentences, want two to four: %q", sentences, desc)
			}
			prop, ok := st.Tool.InputSchema.Properties["selector"].(map[string]any)
			if !ok {
				return
			}
			if desc, _ := prop["description"].(string); !strings.HasSuffix(desc, selectorSyntaxDescription) {
				t.Errorf("selector description %q does not end with the shared selector syntax", desc)
			}
		})
	}
}

// TestToolsDeclareTimeout checks that every tool declares the timeout number
// parameter. Every handler calls ApplyTimeout, which reads timeout from the
// arguments, so a tool added without it accepts an argument it does not declare.
func TestToolsDeclareTimeout(t *testing.T) {
	for _, st := range Registry(nil, nil, nil) {
		t.Run(st.Tool.Name, func(t *testing.T) {
			prop, ok := st.Tool.InputSchema.Properties["timeout"].(map[string]any)
			if !ok {
				t.Fatal("tool does not declare timeout")
			}
			if prop["type"] != "number" {
				t.Errorf("timeout type = %v, want number", prop["type"])
			}
			desc, _ := prop["description"].(string)
			if !strings.Contains(desc, "or the server's configured") {
				t.Errorf("timeout description %q does not name the category default", desc)
			}
		})
	}
}
