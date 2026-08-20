#!/usr/bin/env bash
# Make a new draft post: ask for the title, build the file name from it, and
# write the skeleton — the front matter and the five depth headings.
#
#   scripts/new-draft.sh                  # asks for the title
#   scripts/new-draft.sh "Why rivers meander"
#
# POSTS=other/folder scripts/new-draft.sh "…"   writes somewhere else.
set -euo pipefail

POSTS="${POSTS:-posts}"

title="${*:-}"
if [ -z "$title" ]; then
  printf 'Title: '
  IFS= read -r title
fi

title="$(printf '%s' "$title" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
if [ -z "$title" ]; then
  echo "new-draft: a title is necessary" >&2
  exit 1
fi

# the file name: lower case, one hyphen between words, letters and digits only
slug="$(printf '%s' "$title" \
  | tr '[:upper:]' '[:lower:]' \
  | sed -e "s/['’]//g" -e 's/[^a-z0-9]\+/-/g' -e 's/^-\+//' -e 's/-\+$//')"

if [ -z "$slug" ]; then
  echo "new-draft: that title gives an empty file name" >&2
  exit 1
fi

mkdir -p "$POSTS"
file="$POSTS/$slug.md"
if [ -e "$file" ]; then
  echo "new-draft: $file is already here; nothing was changed" >&2
  exit 1
fi

cat > "$file" <<EOF
---
date: $(date +%F)
tag:
draft: true
---

# $title

# Sentence

# Paragraph

# Short

# Medium

# Long
EOF

echo "$file"
