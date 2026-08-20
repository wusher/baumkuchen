#!/usr/bin/env bash
# Make sure ./posts points at a real folder. The posts live outside this repo,
# so the link is per machine: if it is missing or dangling, ask for the path.
#
#   scripts/link-posts.sh                  # asks, if it needs to
#   scripts/link-posts.sh ~/Documents/blog # no question asked
set -euo pipefail

POSTS="${POSTS:-posts}"

if [ -d "$POSTS" ]; then
  exit 0                                   # a real folder, or a link that works
fi

if [ -L "$POSTS" ]; then
  echo "posts points at $(readlink "$POSTS"), which is not there." >&2
fi

target="${1:-}"
if [ -z "$target" ]; then
  # ask, whether that is a person at a terminal or a path piped in. Nothing
  # to read at all, as in a build with no one watching, ends it here.
  [ -t 0 ] && printf 'Where do the posts live? '
  if ! IFS= read -r target; then
    echo "link-posts: no posts folder, and nothing to read the path from." >&2
    echo "            run: scripts/link-posts.sh /path/to/posts" >&2
    exit 1
  fi
fi

target="${target/#\~/$HOME}"               # a leading ~ is ours to expand
if [ -z "$target" ] || [ ! -d "$target" ]; then
  echo "link-posts: $target is not a folder" >&2
  exit 1
fi
target="$(cd "$target" && pwd -P)"         # absolute, or the link breaks later

rm -f "$POSTS"                             # only ever a link, never a folder
ln -s "$target" "$POSTS"
echo "posts -> $target"
