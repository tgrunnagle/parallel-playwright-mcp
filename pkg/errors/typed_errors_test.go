package errors

import (
	"errors"
	"strings"
	"testing"
)

func TestStaleRefError(t *testing.T) {
	err := NewStaleRefError("f2e7")

	if err.ErrorCode() != CodeElementNotFound {
		t.Errorf("ErrorCode() = %d, want %d", err.ErrorCode(), CodeElementNotFound)
	}
	got := FormatErrorForTool(err)
	want := "[-32002] Stale element ref (reason: ref f2e7 is not on the current page). Take a new snapshot with get_accessibility_tree"
	if got != want {
		t.Errorf("FormatErrorForTool() =\n%q\nwant\n%q", got, want)
	}
	if data := err.ErrorData(); data["suggestion"] != SuggestStaleRef || data["reason"] != "ref f2e7 is not on the current page" {
		t.Errorf("ErrorData() = %v", data)
	}
	if !strings.Contains(err.Error(), "stale element ref") {
		t.Errorf("Error() = %q, want it to name the stale ref", err.Error())
	}
}

func TestMultipleMatchError(t *testing.T) {
	cause := errors.New("strict mode violation")
	err := NewMultipleMatchError("button", 3, []string{`<button> "A"`, `<button> "B"`, `<button> "C"`}, cause)

	got := FormatErrorForTool(err)
	for _, part := range []string{
		"[-32002] Selector matched 3 elements",
		"(selector: button)",
		`(reason: first matches: <button> "A"; <button> "B"; <button> "C")`,
		SuggestMultipleMatch,
	} {
		if !strings.Contains(got, part) {
			t.Errorf("FormatErrorForTool() = %q, want it to contain %q", got, part)
		}
	}
	if !errors.Is(err, cause) {
		t.Error("expected the multi-match error to wrap its cause")
	}

	if noDesc := NewMultipleMatchError("li", 2, nil, nil); noDesc.Reason != "" {
		t.Errorf("Reason = %q, want empty without descriptions", noDesc.Reason)
	}
}

func TestElementNotFoundErrorDefaultsUnchanged(t *testing.T) {
	got := FormatErrorForTool(NewElementNotFoundError("#missing", 0))
	want := "[-32002] Element not found (selector: #missing). " + SuggestElementNotFound
	if got != want {
		t.Errorf("FormatErrorForTool() = %q, want %q", got, want)
	}
}

func TestInteractionError(t *testing.T) {
	cause := errors.New("element is detached")
	err := WrapInteractionError("click", "#go", cause)

	if err.ErrorCode() != CodeInteractionFailed {
		t.Errorf("ErrorCode() = %d, want %d", err.ErrorCode(), CodeInteractionFailed)
	}
	if !errors.Is(err, cause) {
		t.Error("expected the interaction error to wrap its cause")
	}
	if err.Error() != "click failed on #go: element is detached" {
		t.Errorf("Error() = %q", err.Error())
	}

	got := FormatErrorForTool(err)
	want := "[-32005] Interaction failed (selector: #go) (operation: click): element is detached. " + SuggestInteractionFailed
	if got != want {
		t.Errorf("FormatErrorForTool() =\n%q\nwant\n%q", got, want)
	}

	if msg := standardMessage(CodeInteractionFailed); msg != "Interaction failed" {
		t.Errorf("standardMessage(-32005) = %q", msg)
	}
}
