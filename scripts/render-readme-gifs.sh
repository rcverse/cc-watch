#!/bin/zsh

set -euo pipefail

repo_root="${0:A:h}/.."
cd "$repo_root"

vhs tools/ui-demo/cc-watch-demo.tape
vhs tools/ui-demo/cc-watch-keepalive.tape

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/cc-watch-gifs.XXXXXX")"
trap 'rm -rf "$tmp_dir"' EXIT

# Keep the VHS playback, but enlarge the TUI content instead of adding a camera zoom.
magick docs/images/cc-watch-demo.gif -coalesce -background '#16161e' -alpha remove -alpha off -crop 1580x1350+10+0 +repage -resize 1600x1350 -gravity center -extent 1600x1350 -layers Optimize "$tmp_dir/cc-watch-demo-readable.gif"
magick docs/images/cc-watch-keepalive.gif -coalesce -background '#16161e' -alpha remove -alpha off -crop 1580x1350+10+0 +repage -resize 1600x1350 -gravity center -extent 1600x1350 -layers Optimize "$tmp_dir/cc-watch-keepalive-readable.gif"
mv "$tmp_dir/cc-watch-demo-readable.gif" docs/images/cc-watch-demo.gif
mv "$tmp_dir/cc-watch-keepalive-readable.gif" docs/images/cc-watch-keepalive.gif
