#!/usr/bin/env bash
# Render every animation to MP4 in renders/.  Usage:  tools/render-all.sh [--scale 2] [--fps 60]
# Extra flags are passed to record.mjs. Rendering takes about 3 s per second of video at 1080p.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p renders

# Use the system Chromium where Playwright has no build of its own (Ubuntu 26.04).
if [[ -z "${CHROMIUM_PATH:-}" && -x /snap/bin/chromium ]]; then export CHROMIUM_PATH=/snap/bin/chromium; fi

rec() { # rec <page> <seconds> <query> <out>
  if [[ -f "renders/$4" ]]; then echo "skip  $4 (already rendered; delete it to redo)"; return; fi
  echo "▶ $4"
  node tools/record.mjs "$1" --seconds "$2" ${3:+--query "$3"} --out "renders/$4" "${EXTRA[@]}"
}
EXTRA=("$@")

rec cg-montage.html   78 "set=1"            montage-opinions.mp4
rec cg-montage.html   67 "set=2"            montage-feelings.mp4
rec curse-tiers.html  14 ""                 tiers-full.mp4
for t in 0 1 2 3 4; do rec curse-tiers.html 7 "tier=$t" "tier-$t.mp4"; done
rec her-tier.html     59 ""                 her-tier.mp4
rec code-decay.html   37 ""                 code-decay.mp4
rec boss-fight.html   46 ""                 boss-fight.mp4
rec breakup.html      48 "ending=a"         ending-a.mp4
rec breakup.html      32 "ending=b"         ending-b.mp4
rec breakup.html      54 "ending=c"         ending-c.mp4
rec anti-cheat.html   22 "scene=1&noloop"   anti-cheat-1-behind-her-back.mp4
rec anti-cheat.html   29 "scene=2&noloop"   anti-cheat-2-xp-farming.mp4
rec anti-cheat.html   22 "scene=3&noloop"   anti-cheat-3-diary.mp4
for s in 1 2 3 4 5 6 7; do rec git-explained.html 26 "scene=$s" "git-explained-$s.mp4"; done
L=(31 38 28 35 37 36 29 30 29 24 29 34 35)
for s in $(seq 1 13); do rec git-internals.html "${L[$((s-1))]}" "scene=$s&noloop" "$(printf 'git-internals-%02d.mp4' "$s")"; done

echo "Done. Files are in $(pwd)/renders"
