package format

import "testing"

func TestDuration(t *testing.T) {
	cases := []struct {
		sec  int64
		want string
	}{
		{0, "0s"},
		{-5, "0s"}, // clamps negatives
		{45, "45s"},
		{60, "1m 00s"},
		{125, "2m 05s"},
		{3599, "59m 59s"},
		{3600, "1h 00m 00s"},
		{3661, "1h 01m 01s"},
		{36000, "10h 00m 00s"},
	}
	for _, tc := range cases {
		if got := Duration(tc.sec); got != tc.want {
			t.Errorf("Duration(%d) = %q, want %q", tc.sec, got, tc.want)
		}
	}
}

func TestShortID(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"abcd", "abcd"},
		{"12345678", "12345678"},
		{"123456789", "12345678"},
		{"aabbccdd-1234-5678-9abc-deadbeefcafe", "aabbccdd"},
	}
	for _, tc := range cases {
		if got := ShortID(tc.in); got != tc.want {
			t.Errorf("ShortID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
