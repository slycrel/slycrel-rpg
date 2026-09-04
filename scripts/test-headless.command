#!/bin/bash
# The test suite, on a machine with no screen. Double-clickable.
#
# Why this exists: Ebitengine will not initialise without a monitor, and on
# macOS "no monitor" includes a display that has gone to sleep. The whole of
# internal/game fails at package init when that happens — importing the package
# is enough, so even `go test -run XXX` crashes — and the failure is a segfault
# in Ebitengine's own darwin code rather than anything this project can catch.
# There is no headless mode and no environment variable for one.
#
# Linux has Xvfb, an X server that draws into memory and that nobody has to be
# looking at. macOS has no equivalent, which is why this is a container.
#
#   ./scripts/test-headless.command                 # go test ./internal/...
#   ./scripts/test-headless.command go vet ./...    # or anything else
#
# The first run builds the image and compiles Ebitengine's cgo, which takes a
# few minutes; the build cache and the module cache live in named volumes, so
# every run after that is as quick as the host.
set -euo pipefail
cd "$(dirname "$0")/.."

if ! docker info >/dev/null 2>&1; then
    echo "test-headless: Docker is not running. Start Docker Desktop, or run"
    echo "               go test ./internal/... with the screen awake."
    exit 1
fi

if [ -z "$(docker images -q slycrel-headless 2>/dev/null)" ]; then
    echo "Building the headless image (once; a few minutes)..."
    docker build -q -f scripts/headless.Dockerfile -t slycrel-headless . >/dev/null
fi

# The art bundle, which is gitignored and therefore absent from any worktree.
# A worktree gets a symlink to the real checkout's copy instead — and a symlink
# to an absolute host path means nothing inside the container, so the target is
# bind-mounted where the link points. Without this the tests fail on missing
# portraits and effects, which reads as a regression and is a mount.
#
# Declared with a placeholder first element rather than empty, because this runs
# under `set -u` and macOS ships bash 3.2, where expanding an empty array as
# "${art[@]}" is an unbound variable and aborts the script before it reaches
# Docker. The message is `art[@]: unbound variable`, which names the array and
# not the reason, and it fires on every checkout that is not a worktree — which
# is every checkout most of the time.
art=(--rm)
if [ -L assets-raw ]; then
    target=$(cd "$(dirname "$(readlink assets-raw)")" && pwd)/$(basename "$(readlink assets-raw)")
    art=(--rm -v "$target":/src/assets-raw)
fi

run() {
    exec docker run \
        "${art[@]}" \
        -v "$PWD":/src \
        -v slycrel-gocache:/root/.cache/go-build \
        -v slycrel-gomod:/go/pkg/mod \
        slycrel-headless "$@"
}

if [ $# -eq 0 ]; then
    run go test ./internal/...
fi
run "$@"
