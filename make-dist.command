#!/bin/bash
#
# Double-clickable: builds shareable Slycrel for macOS and Windows into dist/.
#
# What comes out is two zips, each holding a binary and the art and audio it
# needs, and nothing else. A friend unzips it and runs it. They need no Go, no
# bundle of their own, and no licence of their own — see docs/ASSET-LICENSING.md
# for why that is the licensed use case rather than an edge around it.
#
# Only the assets the game actually references are copied. assets-raw/ is 16.7
# GB; the 721 files the two manifests point at are 96 MB, and the paths are kept
# exactly as the manifests record them so nothing has to be rewritten.
#
#   ./make-dist.command            # both platforms
#   ./make-dist.command mac        # just one
#   ./make-dist.command windows

set -o pipefail
cd "$(dirname "$0")" || exit 1

# Finder does not start a login shell, so a PATH set up in .zprofile may not be
# here. Same reasoning as play.command.
export PATH="$PATH:/usr/local/go/bin:/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin"

pause() {
	echo
	echo "Press return to close this window."
	read -r _
}

die() {
	echo
	echo "FAILED: $*"
	pause
	exit 1
}

want_mac=1
want_win=1
case "$1" in
mac | macos | darwin) want_win=0 ;;
win | windows) want_mac=0 ;;
"") ;;
*) die "unknown target '$1'. Use mac, windows, or nothing for both." ;;
esac

command -v go >/dev/null 2>&1 || die "Go is not installed, or not on this window's PATH."

VERSION=$(git rev-parse --short HEAD 2>/dev/null || echo "unversioned")
if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
	echo "NOTE: the working tree has uncommitted changes. Building them anyway,"
	echo "      but the build stamp will say $VERSION, which is not quite true."
	echo
fi

STAGE="dist/payload"
rm -rf dist
mkdir -p dist

# ---------------------------------------------------------------------------
# 1. Check the content before shipping it.
#
# The audit is the one that matters here: it walks every art key the game can
# reach and reports the ones that would render as a magenta placeholder. That is
# survivable in development and not something to hand a friend.
# ---------------------------------------------------------------------------
echo "Checking content ..."
go run ./cmd/slycrel -audit >/dev/null 2>&1 || die "the content audit did not pass. Run: go run ./cmd/slycrel -audit"

# ---------------------------------------------------------------------------
# 2. Stage the payload: data, the manifests, and only the files they name.
# ---------------------------------------------------------------------------
echo "Staging assets ..."
mkdir -p "$STAGE/assets"
cp -R data "$STAGE/data" || die "could not copy data/"
cp assets/manifest.json assets/audio.json "$STAGE/assets/" || die "could not copy the manifests"

# The paperwork, which is an obligation rather than a courtesy.
#
# Ebitengine is Apache-2.0, and section 4 requires anyone redistributing the
# work — a compiled binary counts — to hand the recipient a copy of the licence
# and the contents of any NOTICE file. The three golang.org/x modules say the
# same thing in the second clause of BSD-3-Clause. NOTICE has claimed this was
# handled since the day it was written, and it was: by scripts/dist.sh, which is
# not the script that builds releases. This one shipped two of them without a
# line of licence text in either.
[ -d licenses ] || ./scripts/licenses.sh >/dev/null 2>&1
for f in licenses LICENSE NOTICE CREDITS.md docs/ASSET-LICENSING.md; do
	[ -e "$f" ] || die "$f is missing, and a build may not go out without it"
done
cp -R licenses "$STAGE/licenses" || die "could not copy licenses/"
cp LICENSE NOTICE CREDITS.md "$STAGE/" || die "could not copy the licence files"
# The asset audit travels with the build too. It is the document that says which
# of the art may be done what with, and it is no use to anybody sitting in a
# repository the person holding the zip has never seen.
cp docs/ASSET-LICENSING.md "$STAGE/ASSET-LICENSING.md" || die "could not copy the asset audit"

# The file lists come out of the manifests themselves rather than a second copy
# kept here, so a pack added to one cannot be forgotten by the other.
python3 - "$STAGE" <<'PY' || die "staging the assets failed"
import json, os, shutil, sys

stage = sys.argv[1]
wanted = set()
for entry in json.load(open("assets/manifest.json"))["entries"]:
    wanted.add(entry["file"])
for entry in json.load(open("assets/audio.json"))["entries"]:
    wanted.update(entry.get("files", []))

missing, copied, total = [], 0, 0
for rel in sorted(wanted):
    if not os.path.exists(rel):
        missing.append(rel)
        continue
    dst = os.path.join(stage, rel)
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    shutil.copy2(rel, dst)
    copied += 1
    total += os.path.getsize(rel)

if missing:
    print("  %d referenced files are not on disk:" % len(missing))
    for m in missing[:10]:
        print("   ", m)
    raise SystemExit(1)
print("  %d files, %.0f MB" % (copied, total / 1e6))
PY

# What a friend needs to know, and nothing they do not.
cat >"$STAGE/READ ME FIRST.txt" <<'EOF'
SLYCREL

An open-world sword-and-sorcery RPG. Bawdy, absurd, delivered completely
straight. Contains adults behaving exactly as expected.


RUNNING IT ON A MAC

Double-clicking will not work the first time. The build is not signed by an
Apple developer account, so macOS quarantines it and refuses with "cannot be
opened because the developer cannot be verified".

Two ways past that, both entirely normal for a build somebody sent you:

  - Right-click (or Control-click) Slycrel, choose Open, then Open again in the
    dialog that appears. macOS remembers, and afterwards it double-clicks.

  - Or, in Terminal, from this folder:
        xattr -dr com.apple.quarantine .


RUNNING IT ON WINDOWS

Double-click Slycrel.exe. Windows SmartScreen will probably say "Windows
protected your PC" — same reason, no code-signing certificate. Click "More
info", then "Run anyway".


PLAYING IT

Arrows or WASD to move. Z or Enter to confirm, X to go back.
M is the map, C the character sheet, J the journal, H the help screen.

Everything else the game will tell you. Press H first if it does not.


ONE THING WORTH KNOWING

Sleep at an inn. That is what saves the run — not every fight, the way most
games do it. Die on your own and you wake up at the last place you slept, with
everything since undone.

Die with somebody hired and they carry you to the nearest town instead, for a
third of your purse. You keep every yard of progress. That is the entire
argument for hiring anybody, and it is worth more than the extra sword.


WHAT IS IN HERE

The binary, and the art and audio it needs. The art is licensed for exactly
this — being shipped inside a game — and not for being taken out and used as
art. Please do not do that.
EOF

# ---------------------------------------------------------------------------
# 3. Build, and package one folder per platform.
# ---------------------------------------------------------------------------
package() { # name, binary-name, then the build env
	local name="$1" binary="$2"
	local out="dist/$name"
	echo "Packaging $name ..."
	mkdir -p "$out"
	# ditto rather than cp -R: it is the macOS tool for the job and it does not
	# trip over the spaces the asset packs have in their directory names.
	ditto "$STAGE" "$out" || die "could not copy the payload into $out"
	mv "dist/$binary" "$out/$binary" || die "could not place $binary"
	(cd dist && zip -qr "$name.zip" "$name") || die "could not zip $name"
	rm -rf "$out"
	# ls rather than du: du reports blocks allocated on disk, which on APFS came
	# out 16 MB heavier than the file. The number worth printing is the one the
	# person sending it will see.
	echo "  dist/$name.zip  ($(ls -lh "dist/$name.zip" | awk '{print $5}'))"
}

if [ "$want_mac" = 1 ]; then
	echo "Building macOS (universal) ..."
	# Both architectures and then lipo, so one file runs on Apple Silicon and on
	# Intel. A friend should not have to know which machine they have.
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -ldflags "-s -w" -o dist/.slycrel-arm64 ./cmd/slycrel 2>/dev/null ||
		die "the arm64 build failed. Run it by hand to see why: GOOS=darwin GOARCH=arm64 go build ./cmd/slycrel"
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -ldflags "-s -w" -o dist/.slycrel-amd64 ./cmd/slycrel 2>/dev/null ||
		die "the amd64 build failed. Run it by hand to see why: GOOS=darwin GOARCH=amd64 go build ./cmd/slycrel"
	lipo -create -output dist/Slycrel dist/.slycrel-arm64 dist/.slycrel-amd64 || die "lipo could not join the two builds"
	rm -f dist/.slycrel-arm64 dist/.slycrel-amd64
	package "Slycrel-$VERSION-mac" "Slycrel"
fi

if [ "$want_win" = 1 ]; then
	echo "Building Windows ..."
	# No cgo: Ebitengine loads what it needs from DLLs on Windows, which is what
	# makes this cross-buildable from a Mac at all.
	#
	# -H=windowsgui is not optional. Without it Go produces a console binary, so
	# double-clicking opens a terminal window behind the game and leaves it
	# there — which is what the first build of this did. The cost is that panics
	# no longer print anywhere, and for a build going to a friend that is the
	# right side of the trade.
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
		-ldflags "-H=windowsgui -s -w" -o dist/Slycrel.exe ./cmd/slycrel ||
		die "the Windows build failed"
	package "Slycrel-$VERSION-windows" "Slycrel.exe"
fi

rm -rf "$STAGE"

echo
echo "Done. Send somebody a zip from dist/."
echo "They need nothing installed."
pause
