#!/usr/bin/env bash
# Read the posts: what is written, and what is wrong with it.
#
# One row per file — the words in each of the five depths, the total written,
# and the time to read the longest depth — and under a row, whatever is wrong
# with that file. The title carries the verdict: plain when the file is clean,
# amber when it holds a warning, red when it holds an error.
#
#   scripts/audit.sh             # every file in posts/
#   scripts/audit.sh --table     # the numbers only, no findings
#   scripts/audit.sh --words     # the most words first
#   scripts/audit.sh posts/a.md  # one file
#
# It reports:
#   * the depth headings a file does not carry, and a heading with nothing
#     under it
#   * a heading of one # that names no depth, whose text never reaches the page
#   * a missing title, or a missing date or tag
#   * a Sentence depth above 35 words, or a Paragraph depth at 120 or more
#   * a picture it points at that is not in the images folder
#   * a word written twice in a row
#
# It ends with 1 if any file holds an error, and 0 for warnings only. A missing
# depth is a warning: a post may stop early by design, and a draft may hold
# nothing at all. Words are counted the way the site counts them: a picture and
# its caption are not prose, and a code fence does count.
#
# AUDIT_COLOR=always|never   force the colour on or off (a pipe turns it off)
# AUDIT_ICONS=0              plain marks, for a terminal with no nerd font
set -uo pipefail

POSTS="${POSTS:-posts}"
IMAGES="${IMAGES:-$POSTS/images}"

table_only=0
by_words=0
args=()
for a in "$@"; do
  case "$a" in
    --table|-t) table_only=1 ;;
    --words|-w) by_words=1 ;;
    *) args+=("$a") ;;
  esac
done

files=("${args[@]+"${args[@]}"}")
if [ ${#files[@]} -eq 0 ]; then
  # -L, because POSTS may be a symlink to a folder kept somewhere else, and
  # find does not follow a linked starting point on its own
  mapfile -t files < <(find -L "$POSTS" -maxdepth 1 -name '*.md' | sort)
fi

# ---- the colours ----
colour=0
case "${AUDIT_COLOR:-auto}" in
  always) colour=1 ;;
  never)  colour=0 ;;
  *)      [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && colour=1 ;;
esac
if [ "$colour" = 1 ]; then
  HEAD=$'\e[38;5;244m'      # the column names
  TITLE=$'\e[38;5;253m'     # a clean file
  WARNC=$'\e[38;5;179m'     # amber
  ERRC=$'\e[38;5;174m'      # a soft red, easier to read than the plain one
  NUM=$'\e[38;5;249m'
  NONE=$'\e[38;5;238m'      # a depth that is not written
  TOT=$'\e[1;38;5;255m'
  READC=$'\e[38;5;110m'     # a quiet blue
  PINC=$'\e[38;5;74m'
  DIM=$'\e[38;5;242m'
  OK=$'\e[38;5;71m'
  OFF=$'\e[0m'
else
  HEAD=; TITLE=; WARNC=; ERRC=; NUM=; NONE=; TOT=; READC=; PINC=; DIM=; OK=; OFF=
fi
if [ "${AUDIT_ICONS:-1}" = 1 ]; then
  I_HEAD=$''; I_ERR=$''; I_WARN=$''
  I_DRAFT=$''; I_PIN=$''; I_DOT=$'·'; I_SUM=$''
else
  I_HEAD="#"; I_ERR="x"; I_WARN="!"
  I_DRAFT="~"; I_PIN="^"; I_DOT="-"; I_SUM="="
fi

# printf pads by bytes, and a mark from a nerd font may take two columns while
# it counts as one character. Every cell is padded here, by character, and
# nothing that can change width is put to the left of a number.
lpad() { local n=$(( $2 - ${#1} )); [ "$n" -lt 0 ] && n=0; printf '%*s%s' "$n" "" "$1"; }
rpad() { local n=$(( $2 - ${#1} )); [ "$n" -lt 0 ] && n=0; printf '%s%*s' "$1" "$n" ""; }

errors=0; warnings=0; drafts_n=0; files_n=0; grand=0
body_file="$(mktemp)"; count_file="$(mktemp)"
trap 'rm -f "$body_file" "$count_file"' EXIT

# ---- the order of the rows: pinned first, then the newest, as on the index ----
sorted="$(
for f in "${files[@]}"; do
  [ -e "$f" ] || { printf '9\t0000-00-00\t%s\n' "$f"; continue; }
  fm="$(awk 'NR==1 && /^---/ {fm=1; next} fm && /^---/ {exit} fm' "$f")"
  d="$(printf '%s\n' "$fm" | awk -F': *' 'tolower($1)=="date" {print $2; exit}')"
  [ -n "$d" ] || d="$(date -r "$f" +%F 2>/dev/null)"
  [ -n "$d" ] || d="0000-00-00"
  pin=1; printf '%s\n' "$fm" | grep -qiE '^pinned:[[:space:]]*(true|yes|on|1)[[:space:]]*$' && pin=0
  # the total, for --words; the row itself counts again with the full rule
  tot="$(awk '
    NR==1 && /^---[[:space:]]*$/ { fm=1; next }
    fm && /^---[[:space:]]*$/ { fm=0; next }
    fm { next }
    /^# / { seen++; if (seen==1) { cur=""; next }
            h=tolower($0); sub(/^# /,"",h); gsub(/[^a-z0-9]+/," ",h); gsub(/^ +| +$/,"",h)
            if (h=="sentance") h="sentence"
            cur = (h=="sentence"||h=="paragraph"||h=="short"||h=="medium"||h=="long") ? h : ""
            prev=""; next }
    cur == "" { next }
    { t=$0; gsub(/^[ \t]+|[ \t]+$/,"",t)
      if (t ~ /^!\[/) { prev="image"; next }
      if (prev=="image" && t ~ /^\*.*\*$/) { prev=""; next }
      if (t ~ /^:::/) { next }
      prev=""; n += NF }
    END { printf "%08d\n", n }' "$f")"
  printf '%s\t%s\t%s\t%s\n' "$pin" "$d" "$tot" "$f"
done | if [ "$by_words" = 1 ]; then sort -t$'\t' -k3,3r; else sort -t$'\t' -k1,1n -k2,2r; fi | cut -f4
)"

printf '%b%s  %s%b\n\n' "${HEAD}" "$I_HEAD" "$POSTS" "$OFF"
printf '%b%s %s %s %s %s %s %s %s%b\n' "$HEAD" \
  "$(rpad TITLE 30)" "$(lpad SENT 6)" "$(lpad PARA 6)" "$(lpad SHORT 6)" \
  "$(lpad MED 6)" "$(lpad LONG 6)" "$(lpad TOTAL 8)" "$(lpad READ 8)" "$OFF"

rows=""
while IFS= read -r f; do
  [ -n "$f" ] || continue
  files_n=$((files_n + 1))
  found=""; f_err=0; f_warn=0
  err()  { found="$found    ${ERRC}${I_ERR} $1${OFF}\n";   f_err=$((f_err + 1));  errors=$((errors + 1)); }
  warn() { found="$found    ${WARNC}${I_WARN} $1${OFF}\n"; f_warn=$((f_warn + 1)); warnings=$((warnings + 1)); }

  if [ ! -e "$f" ]; then
    err "no such file"
    printf '%b%s%b\n' "$ERRC" "$f" "$OFF"; printf '%b' "$found"
    continue
  fi

  # the body, with the front matter and the fenced code blanked out. A blanked
  # line is kept as an empty line, so every line number is the file's own.
  awk '
    NR==1 && /^---[[:space:]]*$/ { fm=1; print ""; next }
    fm   && /^---[[:space:]]*$/  { fm=0; print ""; next }
    fm { print ""; next }
    /^[[:space:]]{0,3}(```|~~~)/ { fence = !fence; print ""; next }
    fence { print ""; next }
    { print }
  ' "$f" > "$body_file"

  # the same body with the fences kept: the site counts those words
  awk '
    NR==1 && /^---[[:space:]]*$/ { fm=1; next }
    fm   && /^---[[:space:]]*$/  { fm=0; next }
    fm { next }
    { print }
  ' "$f" > "$count_file"

  fm="$(awk 'NR==1 && /^---/ {fm=1; next} fm && /^---/ {exit} fm' "$f")"
  is_draft=0; is_pinned=0
  printf '%s\n' "$fm" | grep -qiE '^draft:[[:space:]]*(true|yes|on|1)[[:space:]]*$'  && is_draft=1
  printf '%s\n' "$fm" | grep -qiE '^pinned:[[:space:]]*(true|yes|on|1)[[:space:]]*$' && is_pinned=1
  [ "$is_draft" = 1 ] && drafts_n=$((drafts_n + 1))

  # ---- the words in each depth, counted the way the site counts ----
  read -r c_sent c_para c_short c_med c_long c_total c_longest < <(awk '
    /^# / { seen++; if (seen == 1) { cur=""; next }
            h=tolower($0); sub(/^# /,"",h); gsub(/[^a-z0-9]+/," ",h);
            gsub(/^ +| +$/,"",h); if (h=="sentance") h="sentence"
            cur = (h=="sentence"||h=="paragraph"||h=="short"||h=="medium"||h=="long") ? h : ""
            prev=""; next }
    cur == "" { next }
    {
      t=$0; gsub(/^[ \t]+|[ \t]+$/,"",t)
      if (t ~ /^!\[/) { prev="image"; next }                 # a picture
      if (prev=="image" && t ~ /^\*.*\*$/) { prev=""; next }  # its caption
      if (t ~ /^:::/) { next }                                # a block marker
      prev=""; n[cur] += NF
    }
    END {
      split("sentence paragraph short medium long", o, " ")
      total=0; longest=0
      for (i=1;i<=5;i++) { v=n[o[i]]+0; total+=v; if (v>longest) longest=v; printf "%d ", v }
      printf "%d %d\n", total, longest
    }' "$count_file")
  grand=$((grand + c_total))

  # ---- a depth counts only when it holds text ----
  have="$(awk '
    function flush() { if (cur != "" && words > 0) print cur }
    /^# / { seen++; flush(); if (seen == 1) { cur=""; next }
            h=tolower($0); sub(/^# /,"",h); gsub(/[^a-z0-9]+/," ",h);
            gsub(/^ +| +$/,"",h); if (h=="sentance") h="sentence"
            cur=h; words=0; next }
    cur != "" { if ($0 !~ /^[ \t]*:::/) words += NF }
    END { flush() }' "$body_file")"
  missing=""
  for name in Sentence Paragraph Short Medium Long; do
    low="$(printf '%s' "$name" | tr '[:upper:]' '[:lower:]')"
    printf '%s\n' "$have" | grep -qx "$low" || missing="$missing $name"
  done
  if [ "$missing" = " Sentence Paragraph Short Medium Long" ]; then
    if [ "$is_draft" -eq 1 ]; then
      warn "no depth yet: it is a draft, so nothing is written under the headings"
    else
      err "no depth at all: this file cannot publish"
    fi
  elif [ -n "$missing" ]; then
    warn "no depth for:$missing"
  fi

  # ---- a heading of one # that names no depth ----
  # The site matches every # after the title against the five names. One that
  # matches nothing takes its text with it: those words never reach the page.
  while read -r stray; do
    [ -n "$stray" ] && err "\"# $stray\" is not a depth name, so its text never reaches the page"
  done < <(awk '
    /^# / { seen++; if (seen == 1) next
            raw=$0; sub(/^# /,"",raw)
            h=tolower(raw); gsub(/[^a-z0-9]+/," ",h); gsub(/^ +| +$/,"",h)
            if (h=="sentence"||h=="sentance"||h=="paragraph"||h=="short"||h=="medium"||h=="long") next
            print raw }' "$body_file")

  # ---- the title ----
  title="$(awk '/^# / { sub(/^# /,""); print; exit }' "$body_file")"
  [ -n "$title" ] || { err "no title: the file needs a first '# ' line"; title="$(basename "$f")"; }

  # ---- the front matter ----
  if [ -z "$fm" ]; then
    warn "no front matter"
  else
    printf '%s\n' "$fm" | grep -qi '^date:'                         || warn "no date: the file time is used"
    printf '%s\n' "$fm" | grep -qiE '^tag:[[:space:]]*[^[:space:]]' || warn "no tag"
  fi

  # ---- the colour, if the file asks for one ----
  # A name that is not one of these leaves the page with the site's accent, so
  # it is worth saying out loud rather than leaving to be noticed.
  colour="$(printf '%s\n' "$fm" | awk -F': *' 'tolower($1)=="color" {print tolower($2); exit}')"
  if [ -n "$colour" ]; then
    case " slate gray zinc neutral stone red orange amber yellow lime green emerald teal cyan sky blue indigo violet purple fuchsia pink rose " in
      *" $colour "*) ;;
      *) err "\"color: $colour\" is not a colour name, so the page keeps the site's accent" ;;
    esac
  fi

  # ---- the depth it opens at, if it names one ----
  opens="$(printf '%s\n' "$fm" | awk -F': *' 'tolower($1)=="depth" {print tolower($2); exit}')"
  if [ -n "$opens" ]; then
    [ "$opens" = "sentance" ] && opens="sentence"
    case " sentence paragraph short medium long " in
      *" $opens "*)
        printf '%s\n' "$have" | grep -qx "$opens" \
          || err "\"depth: $opens\" is not a depth this post carries, so it opens at the nearest one"
        ;;
      *) err "\"depth: $opens\" is not one of the five names" ;;
    esac
  fi

  # ---- the word limits ----
  [ "$c_sent" -gt 35 ]   && err "sentence is $c_sent words, the limit is 35"
  [ "$c_para" -ge 120 ]  && err "paragraph is $c_para words, the limit is 120"

  # ---- the pictures ----
  while read -r img; do
    [ -n "$img" ] && [ ! -e "$IMAGES/$img" ] && err "picture not in $IMAGES: $img"
  done < <(grep -o '](/media/[^)#]*' "$body_file" | sed 's|](/media/||' | sort -u)

  # ---- a word written twice ----
  # Only a true double counts: the two words must touch with nothing but a
  # space between them. "trees, trees" and "with it. It" are ordinary prose,
  # and a few doubles ("had had", "that that") are correct English.
  while read -r line; do
    [ -n "$line" ] && warn "$line"
  done < <(awk '
    BEGIN { split("had that is no so very", ok, " "); for (i in ok) fine[ok[i]] = 1 }
    { for (i = 1; i < NF; i++) {
        a = $i; b = $(i+1)
        if (a !~ /^[A-Za-z]+$/ || b !~ /^[A-Za-z]+$/) continue
        if (tolower(a) == tolower(b) && !(tolower(a) in fine))
          printf "\"%s\" twice in a row, line %d\n", tolower(a), FNR
      } }' "$body_file")

  # ---- the row: the title carries the verdict ----
  tc="$TITLE"
  [ "$f_warn" -gt 0 ] && tc="$WARNC"
  [ "$f_err"  -gt 0 ] && tc="$ERRC"
  short_title="$title"
  [ ${#short_title} -gt 30 ] && short_title="${short_title:0:29}…"

  cell() {
    if [ "$1" -eq 0 ]; then printf '%b%s%b' "$NONE" "$(lpad "$I_DOT" 6)" "$OFF"
    else printf '%b%s%b' "$NUM" "$(lpad "$1" 6)" "$OFF"; fi
  }
  if [ "$c_longest" -eq 0 ]; then read_time="$I_DOT"
  else read_time="$(awk -v w="$c_longest" 'BEGIN { t=w/165; if (t<0.1) print "<1 min"; else printf "%.1f min\n", t }')"
  fi
  mark=""
  [ "$is_draft"  = 1 ] && mark="  ${WARNC}${I_DRAFT} draft${OFF}"
  [ "$is_pinned" = 1 ] && mark="$mark  ${PINC}${I_PIN} pinned${OFF}"

  printf '%b%s%b %s %s %s %s %s %b%s%b %b%s%b%b\n' \
    "$tc" "$(rpad "$short_title" 30)" "$OFF" \
    "$(cell "$c_sent")" "$(cell "$c_para")" "$(cell "$c_short")" \
    "$(cell "$c_med")" "$(cell "$c_long")" \
    "$TOT" "$(lpad "$c_total" 8)" "$OFF" "$READC" "$(lpad "$read_time" 8)" "$OFF" "$mark"
  [ "$table_only" = 0 ] && printf '%b' "$found"
done <<< "$sorted"

# ---- the count ----
pretty="$(printf '%s' "$grand" | sed -e :a -e 's/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/;ta')"
e_col="$OK"; [ "$errors"   -gt 0 ] && e_col="$ERRC"
w_col="$DIM"; [ "$warnings" -gt 0 ] && w_col="$WARNC"
printf '\n%b%s%b %b%d error(s)%b, %b%d warning(s)%b %b·%b %d file(s), %b%d draft(s)%b, %b%s words%b\n' \
  "$DIM" "$I_SUM" "$OFF" "$e_col" "$errors" "$OFF" "$w_col" "$warnings" "$OFF" \
  "$DIM" "$OFF" "$files_n" "$WARNC" "$drafts_n" "$OFF" "$TOT" "$pretty" "$OFF"
[ "$errors" -eq 0 ]
