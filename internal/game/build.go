package game

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/slycrel/slycrel-rpg/internal/gamedata"
)

// BuildStamp is the short description of what is actually running.
//
// It exists because "unless I'm running an old build or something" came up
// while chasing a bug that had already been fixed, and there was no way to
// answer it from inside the game. Two people looking at a screenshot cannot
// tell which commit drew it, and the wrong answer costs an investigation.
//
// Taken from the VCS information the toolchain embeds on its own, so a plain
// `go build` or `go run` carries it with no build flags to remember and nothing
// to keep in step by hand.
var BuildStamp = buildStamp()

func buildStamp() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "build unknown"
	}
	var rev, when string
	dirty := false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.time":
			when = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		// `go run` does not stamp VCS information, and `go run ./cmd/slycrel`
		// is how this game is normally played — so fall back to reading the
		// checkout directly. A distributed binary has the stamp and no
		// checkout; a developer has the checkout and no stamp.
		if r, ok := headRevision(); ok {
			return "build " + r + " (source)"
		}
		return "build unknown"
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	out := "build " + rev
	if d, _, ok := strings.Cut(when, "T"); ok && d != "" {
		out += " " + d
	}
	if dirty {
		out += " +edits"
	}
	return out
}

// headRevision reads the short commit the working tree is on, for the case
// where the toolchain did not embed one.
func headRevision() (string, bool) {
	root, err := gamedata.FindRoot()
	if err != nil {
		return "", false
	}
	head, err := os.ReadFile(filepath.Join(root, ".git", "HEAD"))
	if err != nil {
		return "", false
	}
	line := strings.TrimSpace(string(head))
	// Detached: the file holds the hash. Otherwise it points at a ref.
	if ref, ok := strings.CutPrefix(line, "ref: "); ok {
		b, err := os.ReadFile(filepath.Join(root, ".git", filepath.FromSlash(ref)))
		if err != nil {
			return "", false
		}
		line = strings.TrimSpace(string(b))
	}
	if len(line) < 7 {
		return "", false
	}
	return line[:7], true
}
