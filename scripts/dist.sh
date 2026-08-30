#!/usr/bin/env bash
# Build a shareable Slycrel folder: binary, content tables, and only the art and
# audio the manifests actually reference (~92 MB of the bundle's 16.7 GB).
#
#   ./scripts/dist.sh                 host platform
#   ./scripts/dist.sh windows amd64   cross-compile (no cgo needed)
#   ./scripts/dist.sh darwin arm64
#
# Ebitengine builds for Windows without cgo — it reaches DirectX and the Win32
# API through purego — so a Windows build cross-compiles cleanly from macOS or
# Linux with no toolchain beyond Go itself. macOS and Linux targets DO need cgo
# and therefore must be built on the platform they target.
#
# The art in the output is third-party and separately licensed. Shipping it
# inside a game build is permitted; publishing the folder as an asset pack, or
# committing it, is not. See docs/ASSET-LICENSING.md.
set -euo pipefail

cd "$(dirname "$0")/.."

GOOS_T="${1:-$(go env GOOS)}"
GOARCH_T="${2:-$(go env GOARCH)}"

NAME="slycrel-$GOOS_T-$GOARCH_T"
OUT="dist/$NAME"

BIN="slycrel"
LDFLAGS="-s -w"
CGO=1
if [ "$GOOS_T" = "windows" ]; then
  BIN="slycrel.exe"
  # -H=windowsgui marks it a GUI-subsystem binary so double-clicking does not
  # also open a console window behind the game.
  LDFLAGS="$LDFLAGS -H=windowsgui"
  CGO=0
elif [ "$GOOS_T" != "$(go env GOOS)" ]; then
  echo "error: $GOOS_T needs cgo and cannot be cross-compiled from $(go env GOOS)." >&2
  echo "       Build it on a $GOOS_T machine. Only windows cross-compiles." >&2
  exit 1
fi

rm -rf "$OUT"
mkdir -p "$OUT"

echo "building $GOOS_T/$GOARCH_T..."
GOOS="$GOOS_T" GOARCH="$GOARCH_T" CGO_ENABLED="$CGO" \
  go build -trimpath -ldflags "$LDFLAGS" -o "$OUT/$BIN" ./cmd/slycrel

echo "copying content tables..."
cp -R data "$OUT/"
mkdir -p "$OUT/assets" "$OUT/saves"
cp assets/manifest.json "$OUT/assets/"
[ -f assets/audio.json ] && cp assets/audio.json "$OUT/assets/"

# Copy every file the manifest names, preserving its path so the manifest needs
# no rewriting. rsync -R (--relative) reproduces the assets-raw/... tree.
echo "copying referenced art..."
python3 - "$OUT" <<'PY'
import json, os, shutil, sys
out = sys.argv[1]
seen = set()
for f in ("assets/manifest.json", "assets/audio.json"):
    if not os.path.exists(f):
        continue
    for e in json.load(open(f))["entries"]:
        # art entries carry "file"; audio cues carry "files" (a list of variants)
        if "file" in e:
            seen.add(e["file"])
        seen.update(e.get("files", ()))
n = bytes_ = 0
for p in sorted(seen):
    if not os.path.exists(p):
        print(f"  missing, skipped: {p}")
        continue
    dst = os.path.join(out, p)
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    shutil.copy2(p, dst)
    n += 1
    bytes_ += os.path.getsize(p)
print(f"  {n} files, {bytes_/1024/1024:.1f} MB")
PY

echo "copying licences..."
[ -d licenses ] || ./scripts/licenses.sh >/dev/null
cp -R licenses "$OUT/"
cp LICENSE NOTICE CREDITS.md "$OUT/"

if [ "$GOOS_T" = "windows" ]; then
cat > "$OUT/RUN-ME.txt" <<'EOF'
Slycrel
=======

Double-click slycrel.exe.

Windows SmartScreen will probably say "Windows protected your PC", because the
binary is not code-signed (a certificate costs a few hundred a year, and this is
a weekend project). Click "More info", then "Run anyway". If you would rather
not, that is an entirely reasonable call.

Keep slycrel.exe in this folder -- it loads data/ and assets/ from beside it.

Controls
  arrows / WASD    walk
  Z / Enter        confirm, talk, enter a place
  X / Esc          back out
  M                the map of everywhere you have been
  C or I           character sheet and pack
  Esc              pause: save, load, settings, abandon the run
  \                screenshot, into shots/

Saves land in saves/.

The art and audio belong to their creators and are licensed separately from the
game's own MIT-licensed code. See CREDITS.md and NOTICE.
EOF
else
cat > "$OUT/RUN-ME.txt" <<'EOF'
Slycrel
=======

macOS will refuse to run an unsigned binary downloaded from the internet.
Open Terminal, drag this folder onto the window to cd into it, then:

    xattr -dr com.apple.quarantine .
    ./slycrel

Linux: just ./slycrel

Keep the binary in this folder -- it loads data/ and assets/ from beside it.

Controls
  arrows / WASD    walk
  Z / Enter        confirm, talk, enter a place
  X / Esc          back out
  M                the map of everywhere you have been
  C or I           character sheet and pack
  Esc              pause: save, load, settings, abandon the run
  \                screenshot, into shots/

Saves land in saves/.

The art and audio belong to their creators and are licensed separately from the
game's own MIT-licensed code. See CREDITS.md and NOTICE.
EOF
fi

echo "zipping..."
( cd dist && zip -qr "$NAME.zip" "$NAME" )

echo
echo "done: dist/$NAME.zip  ($(du -h "dist/$NAME.zip" | cut -f1))"
