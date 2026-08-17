#!/bin/bash
#
# Double-clickable launcher: builds this checkout, then plays it.
#
# macOS opens a .command file in Terminal and runs it, which is the only
# double-click that also gets to compile something first. The point is that
# there is no binary to go stale — a stale one cost an evening chasing a bug
# that had already been fixed, with the screenshot and the source disagreeing
# and nothing on screen to say so.
#
# It builds what is in this working tree. It does not pull: this is a checkout
# somebody is working in, and a launcher that quietly moved you to a different
# commit would be a worse version of the problem it exists to solve.
#
# Arguments pass straight through, so from a terminal:
#   ./play.command -seed 1994
#   ./play.command -load saves/fixtures/battered.json

cd "$(dirname "$0")" || exit 1

# Finder does not start a login shell, so a PATH set up in .zprofile is not
# necessarily here. Add the usual places Go ends up.
export PATH="$PATH:/usr/local/go/bin:/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin"

pause() {
	echo
	echo "Press return to close this window."
	read -r _
}

if ! command -v go >/dev/null 2>&1; then
	echo "Go is not installed, or not on the PATH this window got."
	echo "Get it from https://go.dev/dl/ and try again."
	pause
	exit 1
fi

what=$(git rev-parse --short HEAD 2>/dev/null || echo "this working tree")
if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
	what="$what plus uncommitted edits"
fi
echo "Building $what ..."

mkdir -p dist
if ! go build -o dist/slycrel ./cmd/slycrel; then
	echo
	echo "The build failed, so nothing was launched. The error is above."
	pause
	exit 1
fi

./dist/slycrel "$@"
status=$?

# A clean quit closes the window. Anything else stays up, because the reason
# is on screen and closing it is how you lose a crash report.
if [ $status -ne 0 ]; then
	echo
	echo "Slycrel exited with status $status."
	pause
fi
