package tui

import (
	"strings"
	"testing"

	"github.com/AngheloAlva/timer/internal/domain"
)

func TestTaskEditModal_PrefillsTitle(t *testing.T) {
	styles := NewStyles(DefaultPalette)
	m := newTaskEditModel(nil, styles, domain.Task{
		ID:    "abc",
		Title: "Existing title",
	})

	if got := m.titleInput.Value(); got != "Existing title" {
		t.Errorf("prefill = %q, want %q", got, "Existing title")
	}

	out := m.View()
	if !strings.Contains(out, "Edit task title") {
		t.Errorf("missing header: %s", out)
	}
	if !strings.Contains(out, "Existing title") {
		t.Errorf("title not rendered: %s", out)
	}
}
