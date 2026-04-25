// Package projectdetect infers a project slug + display name from the
// MCP process's current working directory. The MCP server is launched
// by the AI client (e.g. Claude Code) inside the user's repo, so cwd
// carries the signal we need.
//
// Ported from apps/mcp/src/project-detection.ts in the Node-era repo.
package projectdetect

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

// Detected bundles every signal we managed to extract from the cwd.
// Empty strings on GitRoot / GitRemoteURL mean "not a git repo / no origin".
type Detected struct {
	Cwd          string
	GitRoot      string
	InferredSlug string
	InferredName string
	GitRemoteURL string
}

// Detect inspects cwd. Pass an empty string to use os.Getwd().
// Always returns a value — callers check the individual fields.
func Detect(cwd string) Detected {
	if cwd == "" {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}
	gitRoot := tryGitRoot(cwd)

	base := cwd
	if gitRoot != "" {
		base = gitRoot
	}
	leaf := filepath.Base(base)

	return Detected{
		Cwd:          cwd,
		GitRoot:      gitRoot,
		InferredSlug: Slugify(leaf),
		InferredName: TitleCase(leaf),
		GitRemoteURL: tryGitRemote(gitRoot),
	}
}

func tryGitRoot(cwd string) string {
	if cwd == "" {
		return ""
	}
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func tryGitRemote(cwd string) string {
	if cwd == "" {
		return ""
	}
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Slugify lowercases and replaces non-alphanumeric runs with single dashes.
// Trims leading/trailing dashes. Caps at 60 chars (matching the Node-era
// behaviour so existing slugs in user DBs stay reachable).
func Slugify(s string) string {
	var b strings.Builder
	prevDash := true
	for _, r := range strings.ToLower(s) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

// TitleCase converts kebab/snake/space-separated words into Title Case.
// "my-cool_project" → "My Cool Project".
func TitleCase(s string) string {
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	fields := strings.Fields(s)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		runes := []rune(f)
		runes[0] = unicode.ToUpper(runes[0])
		out = append(out, string(runes))
	}
	return strings.Join(out, " ")
}
