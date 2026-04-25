package projectdetect

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"timer":              "timer",
		"My Cool Project":    "my-cool-project",
		"my_cool-project":    "my-cool-project",
		"  spaced  ":         "spaced",
		"!@#abc!@#":          "abc",
		"":                   "",
		"a---b":              "a-b",
		"UPPER":              "upper",
		"with123digits":      "with123digits",
		"trailing----dashes": "trailing-dashes",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSlugifyCapsAt60(t *testing.T) {
	long := ""
	for i := 0; i < 80; i++ {
		long += "a"
	}
	got := Slugify(long)
	if len(got) != 60 {
		t.Errorf("expected 60 chars, got %d", len(got))
	}
}

func TestTitleCase(t *testing.T) {
	cases := map[string]string{
		"timer":           "Timer",
		"my-cool-project": "My Cool Project",
		"my_cool_project": "My Cool Project",
		"  hello  world ": "Hello World",
		"":                "",
	}
	for in, want := range cases {
		if got := TitleCase(in); got != want {
			t.Errorf("TitleCase(%q) = %q, want %q", in, got, want)
		}
	}
}

// Detect always returns a value; in a non-git temp dir the slug should
// still come from the basename.
func TestDetectFallbackToBasename(t *testing.T) {
	dir := t.TempDir()
	d := Detect(dir)
	if d.Cwd != dir {
		t.Errorf("Cwd = %q, want %q", d.Cwd, dir)
	}
	if d.InferredSlug == "" {
		t.Error("expected non-empty InferredSlug from temp dir basename")
	}
}
