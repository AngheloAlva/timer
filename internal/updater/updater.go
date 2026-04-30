// Package updater detects how timer was installed and applies in-place
// updates: delegates to Homebrew when installed via brew/cask, or
// downloads the matching release asset from GitHub for standalone
// installs.
package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/minio/selfupdate"
)

const (
	repoOwner = "AngheloAlva"
	repoName  = "timer"
)

type Channel int

const (
	ChannelUnknown Channel = iota
	ChannelBrew
	ChannelStandalone
)

func (c Channel) String() string {
	switch c {
	case ChannelBrew:
		return "homebrew"
	case ChannelStandalone:
		return "standalone"
	default:
		return "unknown"
	}
}

type Release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func DetectChannel() (Channel, string, error) {
	exe, err := os.Executable()
	if err != nil {
		return ChannelUnknown, "", fmt.Errorf("resolve executable path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		resolved = exe
	}
	if isBrewPath(resolved) {
		return ChannelBrew, resolved, nil
	}
	return ChannelStandalone, resolved, nil
}

func isBrewPath(p string) bool {
	p = filepath.ToSlash(p)
	for _, n := range []string{"/Cellar/", "/Caskroom/", "/.linuxbrew/"} {
		if strings.Contains(p, n) {
			return true
		}
	}
	return false
}

func FetchLatest(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "timer-cli")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("github api %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var r Release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}
	return &r, nil
}

// IsNewer reports whether remoteTag (e.g. "v0.4.0") is strictly newer than
// localVersion (e.g. "0.3.0"). "dev" or empty local versions are treated as
// not-upgradeable.
func IsNewer(remoteTag, localVersion string) bool {
	r := strings.TrimPrefix(remoteTag, "v")
	l := strings.TrimPrefix(localVersion, "v")
	if l == "" || l == "dev" {
		return false
	}
	return semverLess(l, r)
}

func semverLess(a, b string) bool {
	aa := splitSemver(a)
	bb := splitSemver(b)
	for i := 0; i < 3; i++ {
		if aa[i] != bb[i] {
			return aa[i] < bb[i]
		}
	}
	return false
}

func splitSemver(v string) [3]int {
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		v = v[:idx]
	}
	var out [3]int
	for i, p := range strings.SplitN(v, ".", 3) {
		if i >= 3 {
			break
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return out
		}
		out[i] = n
	}
	return out
}

// FindAsset returns the release asset matching current GOOS/GOARCH, or nil if
// none matches.
func FindAsset(r *Release) *Asset {
	suffix := ".tar.gz"
	if runtime.GOOS == "windows" {
		suffix = ".zip"
	}
	needle := fmt.Sprintf("_%s_%s%s", runtime.GOOS, runtime.GOARCH, suffix)
	for i := range r.Assets {
		if strings.HasSuffix(r.Assets[i].Name, needle) {
			return &r.Assets[i]
		}
	}
	return nil
}

// UpgradeBrew runs `brew upgrade --cask timer` with stdio streamed to the
// user's terminal.
func UpgradeBrew(ctx context.Context) error {
	brew, err := exec.LookPath("brew")
	if err != nil {
		return fmt.Errorf("brew not found in PATH: %w", err)
	}
	cmd := exec.CommandContext(ctx, brew, "upgrade", "--cask", "timer")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// ApplyStandalone downloads the release asset, extracts the timer binary, and
// replaces the running executable in place. On Windows, selfupdate handles the
// .exe rename trick.
func ApplyStandalone(ctx context.Context, asset *Asset) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "timer-cli")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download asset: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", asset.Name, resp.Status)
	}

	binName := "timer"
	if runtime.GOOS == "windows" {
		binName = "timer.exe"
	}

	binReader, cleanup, err := extractBinary(resp.Body, asset.Name, binName)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := selfupdate.Apply(binReader, selfupdate.Options{}); err != nil {
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return fmt.Errorf("apply update: %w (rollback also failed: %v)", err, rerr)
		}
		return fmt.Errorf("apply update: %w", err)
	}
	return nil
}

// extractBinary streams the matching binary out of the archive. Returns a
// reader and a cleanup func that must be called when the reader is no longer
// used.
func extractBinary(body io.Reader, archiveName, binName string) (io.Reader, func(), error) {
	if strings.HasSuffix(archiveName, ".zip") {
		// zip needs random access; spool to a temp file.
		tmp, err := os.CreateTemp("", "timer-update-*.zip")
		if err != nil {
			return nil, func() {}, err
		}
		size, err := io.Copy(tmp, body)
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
			return nil, func() {}, fmt.Errorf("buffer zip: %w", err)
		}
		zr, err := zip.NewReader(tmp, size)
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
			return nil, func() {}, fmt.Errorf("open zip: %w", err)
		}
		var entry *zip.File
		for _, e := range zr.File {
			if filepath.Base(e.Name) == binName {
				entry = e
				break
			}
		}
		if entry == nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
			return nil, func() {}, fmt.Errorf("binary %q not found in %s", binName, archiveName)
		}
		rc, err := entry.Open()
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
			return nil, func() {}, err
		}
		cleanup := func() {
			_ = rc.Close()
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
		return rc, cleanup, nil
	}

	// tar.gz path: stream-only, no temp file needed.
	gz, err := gzip.NewReader(body)
	if err != nil {
		return nil, func() {}, fmt.Errorf("gunzip: %w", err)
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			_ = gz.Close()
			return nil, func() {}, fmt.Errorf("binary %q not found in %s", binName, archiveName)
		}
		if err != nil {
			_ = gz.Close()
			return nil, func() {}, fmt.Errorf("tar read: %w", err)
		}
		if filepath.Base(h.Name) == binName {
			return tr, func() { _ = gz.Close() }, nil
		}
	}
}
