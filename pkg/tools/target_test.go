package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

func TestResolveTarget(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]any
		optional  bool
		want      elementTarget
		wantError string
	}{
		{
			name: "selector",
			args: map[string]any{"selector": "#go"},
			want: elementTarget{Selector: "#go", Label: "#go"},
		},
		{
			name: "ref",
			args: map[string]any{"ref": "e12"},
			want: elementTarget{Selector: "aria-ref=e12", Ref: "e12", Label: "ref=e12"},
		},
		{
			name: "frame ref",
			args: map[string]any{"ref": "f3e7"},
			want: elementTarget{Selector: "aria-ref=f3e7", Ref: "f3e7", Label: "ref=f3e7"},
		},
		{
			name:      "both",
			args:      map[string]any{"selector": "#go", "ref": "e1"},
			wantError: "[-32602] Invalid parameters (parameter: selector, ref) (reason: provide exactly one of selector or ref, not both)",
		},
		{
			name:      "both on optional",
			args:      map[string]any{"selector": "#go", "ref": "e1"},
			optional:  true,
			wantError: "[-32602] Invalid parameters (parameter: selector, ref)",
		},
		{
			name:      "neither",
			args:      map[string]any{},
			wantError: "[-32602] Invalid parameters (parameter: selector, ref) (reason: exactly one of selector or ref is required)",
		},
		{
			name:      "empty strings count as absent",
			args:      map[string]any{"selector": "", "ref": ""},
			wantError: "[-32602] Invalid parameters (parameter: selector, ref)",
		},
		{
			name:     "neither on optional",
			args:     map[string]any{},
			optional: true,
			want:     elementTarget{},
		},
		{
			name:      "chained selector in ref",
			args:      map[string]any{"ref": "e1 >> css=body"},
			wantError: `[-32602] Invalid parameters (parameter: ref) (reason: "e1 >> css=body" is not an element ref`,
		},
		{
			name:      "other selector engine in ref",
			args:      map[string]any{"ref": "css=div"},
			wantError: `[-32602] Invalid parameters (parameter: ref) (reason: "css=div" is not an element ref`,
		},
		{
			name:      "wrong ref prefix",
			args:      map[string]any{"ref": "x1"},
			wantError: `[-32602] Invalid parameters (parameter: ref) (reason: "x1" is not an element ref`,
		},
		{
			name:      "aria-ref prefix is not a ref",
			args:      map[string]any{"ref": "aria-ref=e1"},
			wantError: "[-32602] Invalid parameters (parameter: ref)",
		},
		{
			name:      "frame without element",
			args:      map[string]any{"ref": "f1"},
			wantError: "[-32602] Invalid parameters (parameter: ref)",
		},
		{
			name:      "trailing newline",
			args:      map[string]any{"ref": "e1\n"},
			wantError: "[-32602] Invalid parameters (parameter: ref)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveTarget(tt.args, tt.optional)
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("resolveTarget() = %+v, want error %q", got, tt.wantError)
				}
				if text := errorResultText(err); !strings.HasPrefix(text, tt.wantError) {
					t.Errorf("error = %q, want prefix %q", text, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveTarget() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveTarget() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestElementTargetLocate(t *testing.T) {
	pageWith := func(loc *mockLocator, located *[]string) *mockInteractionPage {
		return &mockInteractionPage{locatorFunc: func(selector string, _ ...playwright.PageLocatorOptions) playwright.Locator {
			*located = append(*located, selector)
			return loc
		}}
	}

	t.Run("selector is located without a count", func(t *testing.T) {
		var located []string
		counted := false
		loc := &mockLocator{countFunc: func() (int, error) { counted = true; return 0, nil }}
		if _, err := (elementTarget{Selector: "#go", Label: "#go"}).locate(pageWith(loc, &located)); err != nil {
			t.Fatalf("locate() error = %v", err)
		}
		if counted {
			t.Error("selector targets should keep Playwright's own waiting, not count first")
		}
	})

	t.Run("live ref resolves through aria-ref", func(t *testing.T) {
		var located []string
		loc := &mockLocator{countFunc: func() (int, error) { return 1, nil }}
		target, _ := resolveTarget(map[string]any{"ref": "f1e2"}, false)
		if _, err := target.locate(pageWith(loc, &located)); err != nil {
			t.Fatalf("locate() error = %v", err)
		}
		if len(located) != 1 || located[0] != "aria-ref=f1e2" {
			t.Errorf("located %v, want [aria-ref=f1e2]", located)
		}
	})

	t.Run("stale ref fails at once", func(t *testing.T) {
		var located []string
		loc := &mockLocator{countFunc: func() (int, error) { return 0, nil }}
		target, _ := resolveTarget(map[string]any{"ref": "e9"}, false)
		_, err := target.locate(pageWith(loc, &located))
		want := "[-32002] Stale element ref (reason: ref e9 is not on the current page). Take a new snapshot with get_accessibility_tree"
		if got := errorResultText(err); got != want {
			t.Errorf("error = %q, want %q", got, want)
		}
	})

	t.Run("other count failures on a ref are interaction errors, not stale", func(t *testing.T) {
		var located []string
		loc := &mockLocator{countFunc: func() (int, error) {
			return 0, errors.New("target page, context or browser has been closed")
		}}
		target, _ := resolveTarget(map[string]any{"ref": "e9"}, false)
		_, err := target.locate(pageWith(loc, &located))
		got := errorResultText(err)
		if !strings.HasPrefix(got, "[-32005]") || strings.Contains(got, "Stale") || !strings.Contains(got, "has been closed") {
			t.Errorf("error = %q", got)
		}
	})

	t.Run("detached frame on a ref is reported as stale with its cause", func(t *testing.T) {
		var located []string
		loc := &mockLocator{countFunc: func() (int, error) { return 0, errors.New("frame was detached") }}
		target, _ := resolveTarget(map[string]any{"ref": "e9"}, false)
		_, err := target.locate(pageWith(loc, &located))
		got := errorResultText(err)
		if !strings.HasPrefix(got, "[-32002] Stale element ref") || !strings.Contains(got, "frame was detached") {
			t.Errorf("error = %q", got)
		}
	})
}

// TestInteractionHandlersAcceptRef checks every interaction tool acts on a ref
// through aria-ref and reports a stale ref without acting.
func TestInteractionHandlersAcceptRef(t *testing.T) {
	cases := []struct {
		tool    string
		handler func(session.BrowserSessionManager, *TimeoutConfig, *ResponseConfig) server.ToolHandlerFunc
		args    map[string]any
		want    string
	}{
		{"click", ClickHandler, map[string]any{}, "Clicked element: ref=e4"},
		{"type", TypeHandler, map[string]any{"text": "hi"}, "Typed text into element: ref=e4"},
		{"fill", FillHandler, map[string]any{"value": "hi"}, "Filled element: ref=e4"},
		{"select_option", SelectOptionHandler, map[string]any{"value": "a"}, "Selected option 'a' by value in: ref=e4"},
		{"hover", HoverHandler, map[string]any{}, "Hovered over element: ref=e4"},
		{"press_key", PressKeyHandler, map[string]any{"key": "Enter"}, "Pressed key 'Enter' on element: ref=e4"},
	}

	for _, tc := range cases {
		for _, stale := range []bool{false, true} {
			name := tc.tool
			if stale {
				name += " stale"
			}
			t.Run(name, func(t *testing.T) {
				acted := false
				act := func() { acted = true }
				loc := &mockLocator{
					countFunc:             func() (int, error) { return map[bool]int{false: 1, true: 0}[stale], nil },
					clickFunc:             func(playwright.LocatorClickOptions) error { act(); return nil },
					pressSequentiallyFunc: func(string, playwright.LocatorPressSequentiallyOptions) error { act(); return nil },
					fillFunc:              func(string, playwright.LocatorFillOptions) error { act(); return nil },
					hoverFunc:             func(playwright.LocatorHoverOptions) error { act(); return nil },
					pressFunc:             func(string, playwright.LocatorPressOptions) error { act(); return nil },
					selectOptionFunc: func(playwright.SelectOptionValues, playwright.LocatorSelectOptionOptions) ([]string, error) {
						act()
						return []string{"a"}, nil
					},
				}
				var located []string
				page := &mockInteractionPage{locatorFunc: func(selector string, _ ...playwright.PageLocatorOptions) playwright.Locator {
					located = append(located, selector)
					return loc
				}}
				mgr := &mockSessionManager{getSessionFunc: func(string, string) (*session.BrowserSession, bool) {
					return createInteractionSessionWithMockPage(page), true
				}}

				args := map[string]any{"sessionId": "sess-123", "ref": "e4"}
				for k, v := range tc.args {
					args[k] = v
				}
				result, err := tc.handler(mgr, DefaultTimeoutConfig(), nil)(context.Background(), mcp.CallToolRequest{
					Params: mcp.CallToolParams{Name: tc.tool, Arguments: args},
				})
				if err != nil {
					t.Fatalf("handler returned error: %v", err)
				}
				text := extractTextContent(result.Content)

				if stale {
					if !result.IsError || !strings.HasPrefix(text, "[-32002] Stale element ref (reason: ref e4 is not on the current page)") {
						t.Errorf("expected stale ref error, got %q", text)
					}
					if acted {
						t.Error("acted on a stale ref")
					}
					return
				}
				if result.IsError || !strings.HasPrefix(text, tc.want) {
					t.Errorf("result = %q, want prefix %q", text, tc.want)
				}
				if !acted {
					t.Error("expected the action to run")
				}
				if len(located) == 0 || located[0] != "aria-ref=e4" {
					t.Errorf("located %v, want aria-ref=e4 first", located)
				}
			})
		}
	}
}

func TestFirstMatch(t *testing.T) {
	pageWith := func(loc *mockLocator) playwright.Page {
		return &mockInteractionPage{locatorFunc: func(string, ...playwright.PageLocatorOptions) playwright.Locator { return loc }}
	}
	errText := func(err error) string { return extractTextContent(errorResult(err).Content) }

	t.Run("no match fails at once without waiting for an element", func(t *testing.T) {
		waited := false
		page := pageWith(&mockLocator{
			countFunc: func() (int, error) { return 0, nil },
			elementHandleFunc: func() (playwright.ElementHandle, error) {
				waited = true
				return nil, nil
			},
		})
		_, err := firstMatch(page, "role=main", nil)
		if err == nil || !strings.HasPrefix(errText(err), "[-32002]") {
			t.Fatalf("err = %v, want [-32002]", err)
		}
		if waited {
			t.Error("firstMatch waited for an element after a zero count")
		}
	})

	t.Run("count failure is element not found", func(t *testing.T) {
		page := pageWith(&mockLocator{countFunc: func() (int, error) { return 0, errors.New("bad selector") }})
		if _, err := firstMatch(page, "!!", nil); err == nil || !strings.HasPrefix(errText(err), "[-32002]") {
			t.Fatalf("err = %v, want [-32002]", err)
		}
	})

	t.Run("element gone before its handle times out", func(t *testing.T) {
		page := pageWith(&mockLocator{
			countFunc: func() (int, error) { return 1, nil },
			elementHandleFunc: func() (playwright.ElementHandle, error) {
				return nil, errors.New("Timeout 5000ms exceeded")
			},
		})
		timeout := float64(5000)
		if _, err := firstMatch(page, "text=Save", &timeout); err == nil || !strings.HasPrefix(errText(err), "[-32003]") {
			t.Fatalf("err = %v, want [-32003]", err)
		}
	})

	t.Run("match returns the first element", func(t *testing.T) {
		want := &mockElementHandle{}
		page := pageWith(&mockLocator{
			countFunc:         func() (int, error) { return 2, nil },
			elementHandleFunc: func() (playwright.ElementHandle, error) { return want, nil },
		})
		got, err := firstMatch(page, "text=Save", nil)
		if err != nil || got != want {
			t.Fatalf("firstMatch = %v, %v; want the first element", got, err)
		}
	})
}
