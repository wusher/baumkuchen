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
# a draft, held back in a folder whose name begins with an underscore
mkdir -p "$work/posts/_drafts"
cat > "$work/posts/_drafts/hidden.md" <<'EOF'
---
date: 2026-01-03
draft: true
---

# Hidden Draft

# Sentence

A line that is not finished.
EOF

# an empty folder to work in. The settings of the repository are not here, so
# the program reads none of them: the site is published at the root, and the
# links it writes carry no folder.
mkdir -p "$work/away"

# ---- the static site, built with the folder out of reach ----
# No base is given: the settings of the repository are not read out of the
# program, so the program reads none, and the defaults in the code publish the
# site at the root.
( cd "$work/away" && "$binary" build "$work/posts" --out "$work/site" ) >"$work/build.log" 2>&1 \
  || fail "the static build did not finish: $(cat "$work/build.log")"
for f in index.html 404.html static/style.css static/app.js media/pic.jpg post/hello/index.html; do
  [ -f "$work/site/$f" ] || fail "the build did not write $f"
done
grep -q 'href="/post/hello"' "$work/site/index.html" || fail "the built index does not link to the post"
if grep -q '/baumkuchen/' "$work/site/index.html"; then
  fail "the built site carries a folder it was never given"
fi
if [ -f "$work/site/post/hidden/index.html" ]; then
  fail "a draft reached the build"
fi

# the same build in a folder, the way a project page is published
( cd "$work/away" && "$binary" build "$work/posts" --base /baumkuchen --out "$work/site-folder" ) >>"$work/build.log" 2>&1 \
  || fail "the build in a folder did not finish: $(cat "$work/build.log")"
grep -q 'href="/baumkuchen/post/hello"' "$work/site-folder/index.html" \
  || fail "the built index does not carry the folder"

# ---- the check: the same report as text and as JSON ----
( cd "$work/away" && "$binary" check "$work/posts" ) >"$work/check.txt" 2>"$work/check.log" \
  || fail "the check did not finish: $(cat "$work/check.log")"
grep -q '1 published post(s)' "$work/check.txt" || fail "the check does not give the count: $(cat "$work/check.txt")"
( cd "$work/away" && "$binary" check --json "$work/posts" ) >"$work/check.json" 2>"$work/check.log" \
  || fail "the check did not finish: $(cat "$work/check.log")"
# jq reads the count when it is installed and working; the plain read gives
# the same number
if jq --version >/dev/null 2>&1; then
  published="$(jq '.published' <"$work/check.json")"
else
  published="$(sed -n 's/.*"published": *\([0-9]*\).*/\1/p' "$work/check.json" | head -1)"
fi
[ "$published" = "1" ] || fail "the JSON says $published published, want 1"

# ---- the live server, with the folder out of reach ----
# A stylesheet sits in the folder the server runs from. It is not used: the
# files on disk belong to the development mode, and the program stands on its
# own.
mkdir -p "$work/away/static"
printf '/* the smoke test put this here */\nbody { outline: 3px solid red; }\n' \
  > "$work/away/static/style.css"
port=$(( 18000 + ($$ % 900) ))
( cd "$work/away" && exec "$binary" serve "$work/posts" --addr "127.0.0.1:$port" ) >"$work/serve.log" 2>&1 &
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
# the pictures and the stylesheet are not a place to browse
[ "$(code /media/)" = "404" ] || fail "the media folder is listed"
curl -s "http://127.0.0.1:$port/post/hello" | grep -q 'Hello There' \
  || fail "the post page does not hold its title"

# ---- the live part: a post written after the start shows up ----
# The browser is told when the writing changes, and that is on without --dev:
# the page carries the watcher, and the writing has a mark of its own.
# the mark the reload endpoint gives the writing
postsversion() {
  curl -s "http://127.0.0.1:$1/__baumkuchen_reload" \
    | sed -n 's/.*"posts":"\([^"]*\)".*/\1/p'
}

curl -s "http://127.0.0.1:$port/" | grep -q '__baumkuchen_reload' \
  || fail "the ordinary page does not carry the reload script"
before="$(postsversion "$port")"
[ -n "$before" ] || fail "the reload endpoint says nothing about the writing"
[ "$before" = "$(postsversion "$port")" ] \
  || fail "the mark of the writing changed with nothing to change"
printf '\n# Paragraph\n\nA paragraph written after the server started.\n' >> "$work/posts/posts/hello.md"
changed=0
for _ in $(seq 1 50); do
  if curl -s "http://127.0.0.1:$port/post/hello" | grep -q 'written after the server started'; then
    changed=1; break
  fi
  sleep 0.1
done
[ "$changed" = 1 ] || fail "the server did not show the change"
after="$(postsversion "$port")"
[ -n "$after" ] || fail "the reload endpoint says nothing about the writing"
[ "$after" != "$before" ] || fail "the edit to the post did not change the mark"
# the stylesheet on disk is not the one that is served
if curl -s "http://127.0.0.1:$port/static/style.css" | grep -q 'the smoke test put this here'; then
  fail "the ordinary server served a stylesheet from disk"
fi

# ---- the drafts are shown on the loopback address ----
[ "$(code /post/hidden)" = "200" ] || fail "a draft is not shown on the loopback address"

kill "$server" 2>/dev/null || true
server=""

# ---- and held back when the server answers anyone ----
wideport=$(( 19000 + ($$ % 900) ))
( cd "$work/away" && exec "$binary" serve "$work/posts" --addr "0.0.0.0:$wideport" ) >"$work/wide.log" 2>&1 &
server=$!
ready=0
for _ in $(seq 1 50); do
  if curl -s -o /dev/null "http://127.0.0.1:$wideport/"; then ready=1; break; fi
  sleep 0.1
done
[ "$ready" = 1 ] || fail "the server never answered: $(cat "$work/wide.log")"
[ "$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$wideport/post/hidden")" = "404" ] \
  || fail "a draft is shown on an address anyone can reach"

kill "$server" 2>/dev/null || true
server=""

# ---- the default address is the loopback one ----
# No --addr is given, and the settings of the repository are not here, so the
# address in the program's own defaults is used: the loopback one. A server
# that answers only this machine is the safe first answer.
# --open=false is given as well, so that no browser is opened on this
# machine: the flag is read, and nothing is opened.
( cd "$work/away" && exec "$binary" serve "$work/posts" --open=false ) >"$work/default.log" 2>&1 &
server=$!
said=0
for _ in $(seq 1 50); do
  if grep -q 'the blog is on' "$work/default.log" 2>/dev/null; then said=1; break; fi
  if grep -q 'is already taken' "$work/default.log" 2>/dev/null; then said=1; break; fi
  sleep 0.1
done
[ "$said" = 1 ] || fail "the server with the default address never said where it was: $(cat "$work/default.log")"
grep -q '127.0.0.1:8080' "$work/default.log" \
  || fail "the default address is not the loopback one: $(cat "$work/default.log")"
if grep -q 'the blog is on' "$work/default.log"; then
  [ "$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/post/hidden)" = "200" ] \
    || fail "a draft is not shown on the default address"
fi

kill "$server" 2>/dev/null || true
server=""

# ---- the development mode reloads the browser when a template changes ----
mkdir -p "$work/dev"
cp -R "$here/templates" "$here/static" "$work/dev/"
devport=$(( 17500 + ($$ % 400) ))
( cd "$work/dev" && exec "$binary" serve "$work/posts" --addr "127.0.0.1:$devport" --dev ) >"$work/dev.log" 2>&1 &
server=$!
ready=0
for _ in $(seq 1 50); do
  if curl -s -o /dev/null "http://127.0.0.1:$devport/"; then ready=1; break; fi
  sleep 0.1
done
[ "$ready" = 1 ] || fail "the development server never answered: $(cat "$work/dev.log")"
curl -s "http://127.0.0.1:$devport/" | grep -q '__baumkuchen_reload' \
  || fail "the development page does not carry the reload script"
version() {
  curl -s "http://127.0.0.1:$devport/__baumkuchen_reload" \
    | sed -n 's/.*"templates":"\([^"]*\)".*/\1/p'
}
before="$(version)"
[ -n "$before" ] || fail "the reload endpoint says nothing about the templates"
printf '\n<!-- the smoke test changed this -->\n' >> "$work/dev/templates/layout.html"
changed=0
for _ in $(seq 1 50); do
  after="$(version)"
  if [ -n "$after" ] && [ "$after" != "$before" ]; then changed=1; break; fi
  sleep 0.1
done
[ "$changed" = 1 ] || fail "the change to the template was not seen by the browser"

# ---- the stylesheet comes from the disk in the development mode ----
printf '\n/* the smoke test changed this */\nbody { outline: 3px solid red; }\n' \
  >> "$work/dev/static/style.css"
csschanged=0
for _ in $(seq 1 50); do
  if curl -s "http://127.0.0.1:$devport/static/style.css" | grep -q 'the smoke test changed this'; then
    csschanged=1; break
  fi
  sleep 0.1
done
[ "$csschanged" = 1 ] || fail "the development server did not serve the stylesheet from disk"

# ---- an edited post reaches the browser as well ----
# The writing has a version of its own: when a post is edited, the browser is
# told that it changed, and refreshes itself.
postsbefore="$(postsversion "$devport")"
[ -n "$postsbefore" ] || fail "the reload endpoint says nothing about the writing"
printf '\n# Paragraph\n\nA line written while the server was watching.\n' >> "$work/posts/posts/hello.md"
postschanged=0
for _ in $(seq 1 50); do
  postsafter="$(postsversion "$devport")"
  if [ -n "$postsafter" ] && [ "$postsafter" != "$postsbefore" ]; then postschanged=1; break; fi
  sleep 0.1
done
[ "$postschanged" = 1 ] || fail "the edit to the post was not seen by the browser"
curl -s "http://127.0.0.1:$devport/post/hello" | grep -q 'written while the server was watching' \
  || fail "the page does not hold the writing as it is now"

kill "$server" 2>/dev/null || true
server=""

# ---- one file with the program and the posts inside it ----
# The bundle is made from the empty folder, so the settings of the repository
# are not read: the bundle carries the settings that were in effect, which are
# the ones in the code.
( cd "$work/away" && "$binary" bundle "$work/posts" -o "$work/bundle" ) >"$work/bundle.log" 2>&1 \
  || fail "the bundle did not build: $(cat "$work/bundle.log")"
[ -x "$work/bundle" ] || fail "the bundle is not there"
( cd "$work/away" && "$work/bundle" build --out "$work/away/site2" ) >"$work/bundle-build.log" 2>&1 \
  || fail "the bundle did not build the site: $(cat "$work/bundle-build.log")"
[ -f "$work/away/site2/post/hello/index.html" ] || fail "the bundle did not write the post"
[ -f "$work/away/site2/media/pic.jpg" ] || fail "the bundle did not write the picture"
if grep -q '/baumkuchen/' "$work/away/site2/index.html"; then
  fail "the bundle carries a folder it was never given"
fi

# ---- a program with no posts says so, and names the ways to give it some ----
if ( cd "$work/away" && "$binary" build --out "$work/away/site3" ) >"$work/noposts.log" 2>&1; then
  fail "a program with no posts must not build a site"
fi
grep -q 'no posts' "$work/noposts.log" || fail "the error does not name the missing posts: $(cat "$work/noposts.log")"

# ---- every command the README lists is run here ----
# The list is the one in "The commands" of the README, with the folder of the
# writing in place of ~/blog. A command that is written down and is not run
# is a command that does not work.
( cd "$work/away" && "$binary" version ) >"$work/version.log" 2>&1 \
  || fail "the version command did not finish: $(cat "$work/version.log")"
grep -q '^baumkuchen ' "$work/version.log" \
  || fail "the version command said nothing about the build: $(cat "$work/version.log")"

( cd "$work/away" && "$binary" help ) >"$work/help.log" 2>&1 \
  || fail "the help command did not finish: $(cat "$work/help.log")"
grep -q 'baumkuchen \[serve\]' "$work/help.log" \
  || fail "the help does not hold the commands: $(cat "$work/help.log")"

( cd "$work/away" && "$binary" init my-blog ) >"$work/init.log" 2>&1 \
  || fail "the init command did not finish: $(cat "$work/init.log")"
[ -f "$work/away/my-blog/baumkuchen.yml" ] || fail "the init command wrote no settings file"
[ -f "$work/away/my-blog/posts/hello.md" ] || fail "the init command wrote no first post"

# ---- the writing can come from the environment ----
( cd "$work/away" && BAUMKUCHEN_POSTS="$work/posts" "$binary" check ) >"$work/env.log" 2>&1 \
  || fail "the check with the folder in the environment did not finish: $(cat "$work/env.log")"
grep -q '1 published post(s)' "$work/env.log" \
  || fail "the folder in the environment was not used: $(cat "$work/env.log")"

# ---- and the settings can be named on the command line ----
printf 'title: Somewhere Else\n' > "$work/other.yml"
( cd "$work/away" && "$binary" check --config "$work/other.yml" "$work/posts" ) >"$work/config.log" 2>&1 \
  || fail "the check with other settings did not finish: $(cat "$work/config.log")"
grep -q 'other.yml' "$work/config.log" \
  || fail "the settings named on the command line were not used: $(cat "$work/config.log")"

# ---- a flag beats the settings file ----
( cd "$work/away" && "$binary" build "$work/posts" --title "Somewhere Else" --out "$work/site-title" ) \
  >"$work/title.log" 2>&1 \
  || fail "the build with a title did not finish: $(cat "$work/title.log")"
grep -q 'Somewhere Else' "$work/site-title/index.html" \
  || fail "the title on the command line did not reach the built site"

# ---- the command line is read the way it is written ----
code=0
"$binary" --nope >/dev/null 2>&1 || code=$?
[ "$code" = 2 ] || fail "an unknown flag gave $code, want 2"
code=0
"$binary" -export out >/dev/null 2>&1 || code=$?
[ "$code" = 2 ] || fail "the removed export flag gave $code, want 2"
code=0
"$binary" -posts "$work/posts" >/dev/null 2>&1 || code=$?
[ "$code" = 2 ] || fail "the removed posts flag gave $code, want 2"

echo "smoke: the program stands on its own: the static site, the live server,"
echo "smoke: the pictures, the change after the start, the browser told about"
echo "smoke: it, the drafts and the bundle all work."
