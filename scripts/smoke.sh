#!/usr/bin/env bash
# Prove that the finished program stands on its own: the folder it was built
# in is not needed to build the site or to serve it.
#
#   scripts/smoke.sh                    # build the program, then prove it
#   scripts/smoke.sh build/baumkuchen   # prove one that is already built
#
# Everything happens in a temporary folder outside the repository, with posts
# written for the test. Nothing of the real site is read, and nothing of it is
# written.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$here"

fail() { echo "smoke: $1" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || fail "curl is needed for the check"

work="$(mktemp -d)"
server=""
cleanup() {
  [ -n "$server" ] && kill "$server" 2>/dev/null || true
  rm -rf "$work"
}
trap cleanup EXIT

binary="${1:-}"
if [ -n "$binary" ]; then
  # the path is made absolute, so the program can be run from anywhere
  binary="$(cd "$(dirname "$binary")" && pwd)/$(basename "$binary")"
fi
if [ -z "$binary" ]; then
  binary="$work/baumkuchen"
  go build -trimpath -o "$binary" . || fail "the program did not build"
fi
[ -x "$binary" ] || fail "$binary is not there"

# ---- the posts, written for this test ----
mkdir -p "$work/posts/posts" "$work/posts/images"
cat > "$work/posts/posts/hello.md" <<'EOF'
---
date: 2026-01-02
tag: earth
---

# Hello There

# Sentence

A short line about it.
EOF
# a picture the build cannot make smaller, so it is copied as it is
printf 'not really a picture\n' > "$work/posts/images/pic.jpg"

# ---- the static site, built with the folder out of reach ----
mkdir -p "$work/away"
( cd "$work/away" && "$binary" -posts "$work/posts" -base "" -export "$work/site" ) >"$work/export.log" 2>&1 \
  || fail "the static build did not finish: $(cat "$work/export.log")"
for f in index.html 404.html static/style.css static/app.js media/pic.jpg post/hello/index.html; do
  [ -f "$work/site/$f" ] || fail "the build did not write $f"
done
grep -q 'href="/post/hello"' "$work/site/index.html" || fail "the built index does not link to the post"

# the same build in a folder, the way a project page is published
( cd "$work/away" && "$binary" -posts "$work/posts" -base "/baumkuchen" -export "$work/site-folder" ) >>"$work/export.log" 2>&1 \
  || fail "the build in a folder did not finish: $(cat "$work/export.log")"
grep -q 'href="/baumkuchen/post/hello"' "$work/site-folder/index.html" \
  || fail "the built index does not carry the folder"

# ---- the live server, with the folder out of reach ----
port=$(( 18000 + ($$ % 900) ))
"$binary" -posts "$work/posts" -addr "127.0.0.1:$port" >"$work/serve.log" 2>&1 &
server=$!
ready=0
for _ in $(seq 1 50); do
  if curl -s -o /dev/null "http://127.0.0.1:$port/"; then ready=1; break; fi
  sleep 0.1
done
[ "$ready" = 1 ] || fail "the server never answered: $(cat "$work/serve.log")"

code() { curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$port$1"; }
[ "$(code /)" = "200" ] || fail "the index did not answer"
[ "$(code /post/hello)" = "200" ] || fail "the post did not answer"
[ "$(code /static/style.css)" = "200" ] || fail "the stylesheet did not answer"
[ "$(code /media/pic.jpg)" = "200" ] || fail "the picture did not answer"
[ "$(code /post/nothing-here)" = "404" ] || fail "a page that is not there answered"
[ "$(code /not-a-page)" = "404" ] || fail "a stray address answered"
curl -s "http://127.0.0.1:$port/post/hello" | grep -q 'Hello There' \
  || fail "the post page does not hold its title"

# ---- the live part: a post written after the start shows up ----
printf '\n# Paragraph\n\nA paragraph written after the server started.\n' >> "$work/posts/posts/hello.md"
sleep 0.2
curl -s "http://127.0.0.1:$port/post/hello" | grep -q 'written after the server started' \
  || fail "the server did not show the change"

kill "$server" 2>/dev/null || true
server=""

# ---- one file with the program and the posts inside it ----
"$binary" -bundle "$work/posts" -o "$work/bundle" >"$work/bundle.log" 2>&1 \
  || fail "the bundle did not build: $(cat "$work/bundle.log")"
[ -x "$work/bundle" ] || fail "the bundle is not there"
rm -rf "$work/away/site2"
( cd "$work/away" && "$work/bundle" -export "$work/away/site2" ) >"$work/bundle-export.log" 2>&1 \
  || fail "the bundle did not build the site: $(cat "$work/bundle-export.log")"
[ -f "$work/away/site2/post/hello/index.html" ] || fail "the bundle did not write the post"
[ -f "$work/away/site2/media/pic.jpg" ] || fail "the bundle did not write the picture"

# ---- a program with no posts says so, and names the ways to give it some ----
if ( cd "$work/away" && "$binary" -export "$work/away/site3" ) >"$work/noposts.log" 2>&1; then
  fail "a program with no posts must not build a site"
fi
grep -q 'no folder at' "$work/noposts.log" || fail "the error does not name the missing folder"

echo "smoke: the program stands on its own: the static site, the live server,"
echo "smoke: the pictures, the change after the start, and the bundle all work."
