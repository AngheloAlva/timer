package updater

import (
	"runtime"
	"testing"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		remote string
		local  string
		want   bool
	}{
		{"v0.4.0", "0.3.0", true},
		{"v0.3.1", "0.3.0", true},
		{"0.3.0", "0.3.0", false},
		{"v0.3.0", "v0.3.0", false},
		{"v0.2.9", "0.3.0", false},
		{"v0.10.0", "0.9.0", true},
		{"v1.0.0", "0.99.99", true},
		{"v0.3.0", "dev", false},
		{"v0.3.0", "", false},
		{"v0.4.0-rc1", "0.3.0", true},
	}
	for _, c := range cases {
		if got := IsNewer(c.remote, c.local); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.remote, c.local, got, c.want)
		}
	}
}

func TestIsBrewPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/opt/homebrew/Cellar/timer/0.3.0/bin/timer", true},
		{"/opt/homebrew/Caskroom/timer/0.3.0/timer", true},
		{"/usr/local/Cellar/timer/0.3.0/bin/timer", true},
		{"/home/linuxbrew/.linuxbrew/Cellar/timer/0.3.0/bin/timer", true},
		{"/home/anghelo/go/bin/timer", false},
		{"/usr/local/bin/timer", false},
		{"C:\\Users\\anghelo\\scoop\\apps\\timer\\0.3.0\\timer.exe", false},
		{"/tmp/timer", false},
	}
	for _, c := range cases {
		if got := isBrewPath(c.path); got != c.want {
			t.Errorf("isBrewPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestFindAsset(t *testing.T) {
	r := &Release{
		Assets: []Asset{
			{Name: "checksums.txt"},
			{Name: "timer_0.3.0_darwin_amd64.tar.gz"},
			{Name: "timer_0.3.0_darwin_arm64.tar.gz"},
			{Name: "timer_0.3.0_linux_amd64.tar.gz"},
			{Name: "timer_0.3.0_linux_arm64.tar.gz"},
			{Name: "timer_0.3.0_windows_amd64.zip"},
			{Name: "timer_0.3.0_windows_arm64.zip"},
		},
	}
	got := FindAsset(r)
	if got == nil {
		t.Fatalf("FindAsset returned nil for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	wantSuffix := "_" + runtime.GOOS + "_" + runtime.GOARCH
	if !contains(got.Name, wantSuffix) {
		t.Errorf("FindAsset = %q, want to contain %q", got.Name, wantSuffix)
	}
}

func TestFindAsset_NoMatch(t *testing.T) {
	r := &Release{Assets: []Asset{{Name: "checksums.txt"}}}
	if got := FindAsset(r); got != nil {
		t.Errorf("FindAsset = %v, want nil", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub ||
		(len(sub) > 0 && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
