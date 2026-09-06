#!/usr/bin/env bash
# ui-shots.sh — task 15.4: screenshot matrix for every route in scripts/ui/routes.txt.
#
#   scripts/ui/ui-shots.sh all            # 4 states per route: 1440 light, 1440 dark, 390 light, 390 dark
#   scripts/ui/ui-shots.sh desktop        # 1440 light + dark only
#   scripts/ui/ui-shots.sh mobile         # 390 light + dark only
#   UI_SHOTS_ONLY=keys,usage scripts/ui/ui-shots.sh all   # subset by slug
#
# Output: screens/<slug>/<state>.png (state ∈ d-light d-dark m-light m-dark). Requires the mock
# backend (:8091) and Vite (:3777 with VITE_DEV_PROXY_TARGET=http://127.0.0.1:8091) to be running.
set -uo pipefail
cd "$(dirname "$0")/../.."

mode="${1:-all}"
routes_file="${UI_SHOTS_ROUTES:-scripts/ui/routes.txt}"
out_root="${UI_SHOTS_OUT:-screens}"
only="${UI_SHOTS_ONLY:-}"

case "$mode" in
  all)     states="d-light d-dark m-light m-dark" ;;
  desktop) states="d-light d-dark" ;;
  mobile)  states="m-light m-dark" ;;
  *) echo "usage: $0 all|desktop|mobile" >&2; exit 2 ;;
esac

for p in 8091 3777; do
  if ! (exec 3<>/dev/tcp/127.0.0.1/$p) 2>/dev/null; then
    echo "port $p not listening — start the mock server / Vite first" >&2; exit 1
  fi
done

total=0; failed=0
while IFS='|' read -r pattern role actual; do
  [[ -z "$pattern" || "$pattern" == \#* ]] && continue
  actual="${actual:-$pattern}"; role="${role:-guest}"
  slug="$(printf '%s' "$actual" | sed -E 's#^/##; s#[^A-Za-z0-9]+#-#g; s#-+$##')"
  [[ -z "$slug" ]] && slug=root
  if [[ -n "$only" ]] && ! grep -qx "$slug" <<<"${only//,/$'\n'}"; then continue; fi
  mkdir -p "$out_root/$slug"
  for st in $states; do
    case "$st" in
      d-light) w=1440; h=900; theme=glass-light ;;
      d-dark)  w=1440; h=900; theme=glass-dark ;;
      m-light) w=390;  h=844; theme=glass-light ;;
      m-dark)  w=390;  h=844; theme=glass-dark ;;
    esac
    total=$((total+1))
    if UI_SHOTS_DIR="$out_root/$slug" node scripts/ui/shot.cjs "$st" "$actual" "$w" "$h" "$role" "$theme" zh 1 >/dev/null 2>&1; then
      echo "✓ $slug/$st"
    else
      failed=$((failed+1)); echo "✗ $slug/$st" >&2
    fi
  done
done < "$routes_file"

echo "shots: $total, failed: $failed"
[[ $failed -eq 0 ]]
