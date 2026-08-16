#!/usr/bin/env bash
# Vendor the licence text of every third-party module compiled into the game.
#
#   ./scripts/licenses.sh     -> licenses/<module>/LICENSE + licenses/THIRD-PARTY.md
#
# This is a compliance requirement, not a courtesy. Ebitengine is Apache-2.0,
# whose section 4 obliges anyone redistributing the work — including inside a
# compiled binary — to ship a copy of the licence. The BSD-3-Clause modules
# carry the same obligation in their second clause. Run this after any
# dependency change and commit the result.
set -euo pipefail

cd "$(dirname "$0")/.."

MC="$(go env GOMODCACHE)"
OUT="licenses"

rm -rf "$OUT"
mkdir -p "$OUT"

# Only modules whose packages are actually linked into the game binary; this
# excludes build- and test-only dependencies that are never shipped.
mods="$(go list -deps ./cmd/slycrel |
        xargs go list -f '{{if .Module}}{{.Module.Path}}@{{.Module.Version}}{{end}}' 2>/dev/null |
        grep -v '^github.com/slycrel/slycrel-rpg' |
        sort -u)"

{
  echo "# Third-party licences"
  echo
  echo "Every module compiled into the Slycrel binary, with its licence text"
  echo "vendored beside this file. Regenerate with \`./scripts/licenses.sh\`."
  echo
  echo "This covers **code only**. The art and audio are licensed separately and"
  echo "are not part of this repository — see [../CREDITS.md](../CREDITS.md) and"
  echo "[../docs/ASSET-LICENSING.md](../docs/ASSET-LICENSING.md)."
  echo
  echo "| module | version | licence |"
  echo "|---|---|---|"
} > "$OUT/THIRD-PARTY.md"

for m in $mods; do
  path="${m%@*}"
  ver="${m##*@}"
  # The module cache escapes capitals as !lowercase.
  esc="$(echo "$path" | sed 's|\([A-Z]\)|!\l\1|g')"
  src="$MC/$esc@$ver"

  file=""
  for f in LICENSE LICENSE.md LICENSE.txt COPYING COPYING.md; do
    if [ -f "$src/$f" ]; then file="$f"; break; fi
  done
  if [ -z "$file" ]; then
    echo "  WARNING: no licence file found for $m" >&2
    echo "| \`$path\` | $ver | **not found — check manually** |" >> "$OUT/THIRD-PARTY.md"
    continue
  fi

  mkdir -p "$OUT/$path"
  cp "$src/$file" "$OUT/$path/LICENSE"

  # Identify the licence from its first lines.
  kind="see LICENSE"
  head -3 "$src/$file" | grep -qi "Apache License" && kind="Apache-2.0"
  head -5 "$src/$file" | grep -qi "Redistribution and use in source and binary" && kind="BSD-3-Clause"
  head -3 "$src/$file" | grep -qi "MIT License" && kind="MIT"
  head -3 "$src/$file" | grep -qi "ISC License" && kind="ISC"
  # SIL OFL, used by bundled bitmap fonts.
  head -3 "$src/$file" | grep -qi "SIL OPEN FONT LICENSE" && kind="OFL-1.1"
  # Some modules offer a choice; prefer the declared SPDX expression. The
  # `|| true` matters: grep exits 1 when a licence declares no SPDX line, and
  # a bare failing assignment would trip `set -e` and abort the whole run.
  spdx="$(grep -m1 -o 'SPDX-License-Identifier: .*' "$src/$file" 2>/dev/null | cut -d' ' -f2- || true)"
  [ -n "$spdx" ] && kind="$spdx"

  echo "| \`$path\` | $ver | $kind |" >> "$OUT/THIRD-PARTY.md"
  echo "  $path  ($kind)"
done

echo
echo "wrote $OUT/THIRD-PARTY.md and $(find "$OUT" -name LICENSE | wc -l | tr -d ' ') licence files"
