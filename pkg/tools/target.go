// Package tools contains MCP tool implementations for browser automation.
package tools

import (
	"fmt"

	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/errors"
)

// targetParams names both targeting parameters in validation errors.
const targetParams = "selector, ref"

// refParamDescription describes the ref argument of the interaction tools.
const refParamDescription = "Element ref from get_accessibility_tree (or an action's includeSnapshot), such as e3 or f1e2. Use instead of selector"

// elementTarget is the element an interaction tool acts on: a selector, or an
// element ref from a snapshot.
type elementTarget struct {
	// Selector is the Playwright selector that locates the element: the caller's
	// selector, or aria-ref=<ref> for a ref.
	Selector string
	// Ref is the element ref, empty for a selector target.
	Ref string
	// Label names the target in messages: the selector, or ref=<ref>.
	Label string
}

// isZero reports whether no target was given.
func (t elementTarget) isZero() bool {
	return t.Selector == ""
}

// resolveTarget reads the selector and ref arguments. Exactly one is required;
// when optional is true, neither is also accepted and yields the zero target. A
// ref must have the e<n> or f<k>e<n> form, so it cannot smuggle in another selector
// engine or a chained selector. Validation happens before any page access.
func resolveTarget(args map[string]any, optional bool) (elementTarget, error) {
	selector, _ := args["selector"].(string)
	ref, _ := args["ref"].(string)

	switch {
	case selector != "" && ref != "":
		return elementTarget{}, errors.NewInvalidParamsError(targetParams, "provide exactly one of selector or ref, not both")
	case selector == "" && ref == "":
		if optional {
			return elementTarget{}, nil
		}
		return elementTarget{}, errors.NewInvalidParamsError(targetParams, "exactly one of selector or ref is required")
	case ref != "":
		if !refPattern.MatchString(ref) {
			return elementTarget{}, errors.NewInvalidParamsError("ref",
				fmt.Sprintf("%q is not an element ref; refs look like e3 or f1e2", ref))
		}
		return elementTarget{Selector: refSelector(ref), Ref: ref, Label: "ref=" + ref}, nil
	default:
		return elementTarget{Selector: selector, Label: selector}, nil
	}
}

// locate returns the locator for the target. A ref that resolves to no element
// fails at once with the stale-ref error, instead of waiting out the timeout.
func (t elementTarget) locate(page playwright.Page) (playwright.Locator, error) {
	loc := page.Locator(t.Selector)
	if t.Ref == "" {
		return loc, nil
	}
	count, err := loc.Count()
	if err != nil {
		stale := errors.NewStaleRefError(t.Ref)
		stale.Err = err
		return nil, stale
	}
	if count == 0 {
		return nil, errors.NewStaleRefError(t.Ref)
	}
	return loc, nil
}
