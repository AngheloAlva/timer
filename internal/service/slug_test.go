package service

import "testing"

func TestSlugify(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Timer CLI", "timer-cli"},
		{"  Timer CLI  ", "timer-cli"},
		{"Side Hustle", "side-hustle"},
		{"already-kebab", "already-kebab"},
		{"snake_case", "snake-case"},
		{"Mixed_Case-Stuff", "mixed-case-stuff"},
		{"123 numbers ok", "123-numbers-ok"},
		{"", ""},
		{"   ", ""},
		{"!@#$%^&*()", ""},
		{"Hola Café", "hola-caf"}, // non-ASCII letters dropped
	}

	for _, tc := range cases {
		got := Slugify(tc.in)
		if got != tc.want {
			t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
