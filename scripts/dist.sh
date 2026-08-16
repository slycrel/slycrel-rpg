#!/usr/bin/env bash
# Build a shareable Slycrel folder: binary, content tables, and only the art the
# manifest actually references (~52 MB of the bundle's 16.7 GB).
#
#   ./scripts/dist.sh          -> dist/slycrel-<os>-<arch>/ and a .zip beside it
#
# The art in the output is third-party and separately licensed. Shipping it
# inside a game build is permitted; publishing the folder as an asset pack, or
# committing it, is not. See docs/ASSET-LICENSING.md.
set -euo pipefail

cd "$(dirname "$0")/.."

NAME="slycrel-$(go env GOOS)-$(go env GOARCH)"
OUT="dist/$NAME"

rm -rf "$OUT"
mkdir -p "$OUT"

echo "building..."
go build -trimpath -ldflags "-s -w" -o "$OUT/slycrel" ./cmd/slycrel

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

cp README.md LICENSE NOTICE CREDITS.md "$OUT/" 2>/dev/null || true

cat > "$OUT/RUN-ME.txt" <<'EOF'
Slycrel
=======

macOS:   open Terminal, drag this folder in, then run:  ./slycrel
Linux:   ./slycrel
Windows: slycrel.exe

macOS will refuse to run an unsigned binary downloaded from the internet.
Clear the quarantine flag once:

    xattr -dr com.apple.quarantine .

Controls: arrows/WASD to walk, Z to confirm, X to back out, M for the map,
C for the character sheet, Esc to pause and save.

The art and audio belong to their creators and are licensed separately from
the game's own MIT-licensed code. See CREDITS.md and NOTICE.
EOF

echo "zipping..."
( cd dist && zip -qr "$NAME.zip" "$NAME" )

echo
echo "done: dist/$NAME.zip  ($(du -h "dist/$NAME.zip" | cut -f1))"
