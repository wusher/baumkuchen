# Four Depths

A small Go blog that serves markdown from `posts/`. A post carries the same
idea at up to five lengths, and a slider moves the reader between the ones it
has. New words fill the page as the text grows, and drain away as it shrinks.

## Run it

    make setup     # download the Go modules
    make run       # http://localhost:8080
    make new       # start a new draft; it asks for the title
    make audit     # check the posts before you publish
    make stats     # the words in each depth, and the time to read
    make export    # build the static site into ./dist
    make lint      # repair the format, then vet
    make build     # a single binary, templates and assets included
    make release   # the finished program, with its name in it
    make install   # build and install into ~/.local/bin
    make smoke     # prove the finished program stands on its own

## The program on its own

The program is one file. The templates, the stylesheet, the script and the
settings are built into it, so a copy of it runs anywhere: it needs the folder
it was made in for nothing at all.

    make release                     # ./build/baumkuchen
    ./build/baumkuchen -version

The **posts** are not built in. They are the writing, and they stay where the
writer keeps them. The program is told where they are:

    ./build/baumkuchen -posts ~/blog                  # this folder
    BAUMKUCHEN_POSTS=~/blog ./build/baumkuchen         # the same, from the environment
    ./build/baumkuchen -posts ~/blog -config my.yml    # with other settings

The first one that is there wins: the flag, then the environment, then the
`posts:` line in the settings file. A folder that is not there is reported by
name, rather than quietly giving an empty site.

There are two ways to run it.

**The static site.** The build writes the whole site as plain files and stops.

    ./build/baumkuchen -posts ~/blog -export out

**The live site.** The server answers on an address and rereads the posts when
a file changes, so you can edit a post and refresh the page.

    ./build/baumkuchen -posts ~/blog -addr 127.0.0.1:8080

Both work the same whether the posts are in a folder or inside the program.

### One file with the posts inside

A copy of the program with the posts in it is made on purpose, by naming the
folder to put in. Nothing is ever put inside a program by accident.

    make bundle CONTENT=~/blog          # ./build/baumkuchen-bundle
    ./build/baumkuchen-bundle -export out

That one file builds the site and serves it, with nothing else on the machine:
the posts, the templates, the stylesheet and the settings are all in it.

### Install

    make install                       # ~/.local/bin/baumkuchen
    baumkuchen -posts ~/blog

Make sure `~/.local/bin` is on your `PATH`. Only the program is installed;
posts stay in the folder you choose. Override `PREFIX` for another location,
or use `DESTDIR` to stage an installation for packaging:

    make install PREFIX=/usr/local DESTDIR=/tmp/package

### The build

    make build        # ./baumkuchen, for the work in progress
    make release      # ./build/baumkuchen, with the version in it
    make releases     # the same, for linux, darwin and windows, on both chips
    make smoke        # prove the program stands on its own

`make release` writes the name of the build into the program, so a copy can
say which one it is:

    ./build/baumkuchen -version
    baumkuchen v1.2.3 (a1b2c3d, 2026-09-01T10:00:00Z)

The build is the same every time, given the same source and the same tool
chain. `SOURCE_DATE_EPOCH` fixes the day, so two builds on two days give the
same file:

    SOURCE_DATE_EPOCH=1759312800 make release

`make smoke` is the check that matters: it copies the finished program out of
the repository, writes some posts for the test, builds the site, serves it,
changes a post and sees the change, and does the same with a copy that has the
posts inside. The tests in the repository do the same in Go.

## The settings

`baumkuchen.yml`, at the top of the tree. Every setting has a working default,
so the file can be short, or missing altogether.

    title: Four Depths     # the tab, and the wordmark
    posts: posts           # where the markdown is read from
    dist:  dist            # where the built site is written
    base:  /baumkuchen     # the folder it is published in; empty for the root
    addr:  :8080           # where the server listens

A flag beats the file, and only a flag you actually gave:

    go run . -addr :3000            # this once, on another port
    go run . -title "Something Else"
    go run . -config other.yml

A key that is not a setting stops the run and names itself, because a typo
that is swallowed is a setting that quietly does nothing. The Makefile reads
the folders out of the same file, so there is one place to change them.

## Where the posts live

Every folder under `posts/` is read, however deep. The folders are for you, not
for the site: the file name is still the URL, so `posts/2026/why-rivers.md`
answers at `/post/why-rivers`.

The folder is wherever the writing is. In this repository it is a link to the
folder outside, and the link is followed once, when the folder is opened, so
every path inside is a real one. A program that was copied somewhere else
reads the same posts from the folder you name with `-posts`.

Two folders are stepped over: `images/`, which holds the pictures, and anything
whose name starts with a dot.

**A folder whose name starts with `_` holds its files back.** Everything under
`_drafts/` is a draft, whatever its own front matter says — the folder is you
saying "not yet". Such a file is on the index and readable while the server
runs, marked as a draft, and never in the built site.

Two files with the same name in different folders would answer at one URL. The
first one in the walk keeps it, and the startup log names both, so the clash is
never silent.

## The publishing rule

A file in `posts/` becomes a post when it holds **one or more** of the five
depth headings. A file with none of them stays invisible: it is not on the
index, and its URL gives a 404. Nothing else controls publication.

A post keeps the order below, whichever depths it carries. It can stop early,
or skip a depth in the middle. With one depth only, the page shows no slider.

```markdown
---
date: 2026-08-14
tag: materials
---

# The Title

# Sentence
# Paragraph
# Short
# Medium
# Long
```

### The five depths

Every heading of one `#` is a marker. The **first** one is the title of the
post; each one after it names a depth. A heading of `##` or deeper is ordinary
text inside the depth it sits in.

| Heading | Depth | The length it holds |
|---|---|---|
| `# Sentence` | Sentence | One sentence. **35 words** at most. |
| `# Paragraph` | Paragraph | One paragraph, under **120 words**. |
| `# Short` | Short | About three paragraphs. |
| `# Medium` | Medium | About one page. |
| `# Long` | Long | About three pages. |

The slider follows that order, top to bottom. The `Depth` column is the name
the reader sees above the slider.

* Each depth is a whole version of the same topic at that length, not a
  continuation of the one above it. A reader can arrive at any depth first.
* Keep it brief. The `Sentence` depth stays at or under **35 words**, and a
  paragraph runs under **120 words** — at most one paragraph in four may go over.
  `post_test.go` enforces both across `posts/`.
* Use the markdown: bold and italics for terms, `##` inside a long depth,
  bold run-in heads inside a medium one, lists, tables, code fences, images.
* The first `# ` line is the title. The file name is the URL.
* The index shows the `Sentence` depth, or the shortest depth a post has.
* Pictures go in `posts/images/` and are written as `![alt](/media/name.jpg)`.
  A line of `*italic*` under the picture becomes its caption. No build is
  needed: the folder is served from disk.
* End the path with `#left`, `#right` or `#center` to place a picture:
  `![alt](/media/name.jpg#right)`. `#left` and `#right` float it and the text
  wraps beside it; `#center` sits it in the middle with no wrap. All three are
  held to **half the article width**, and the caption travels with the picture.
  A path with no modifier runs the full width. Below 620px every modifier falls
  back to full width.
* Text set in the middle of the column, two ways. A marker at the head of a
  line, the way `>` marks a quotation, for a line or two:

      -> **one line** in the middle
      -> and the next one with it

      an ordinary paragraph, which needs no closing marker

  And a fence, when a whole group of blocks belongs in the middle:

      ::: center
      ## A heading, a picture and a list
      ![alt](/media/frame.jpg)
      - and a list with them
      :::

  Both keep ordinary markdown inside them, so a link is still a link. Neither
  marker is a word, so they do not count against the 35 and the 120. An arrow
  run ends at the first line without an arrow; a fence left open at the end of
  a depth is closed for you; and inside a code fence both are only text.

  Plain HTML works as well, and is what other markdown tools understand:
  `<div align="center">`, a blank line, the markdown, a blank line, `</div>`.
* Fenced code blocks work, and are shown as one piece.
* The front matter is optional. Without a `date`, the file time is used.
* Headings are matched without case or punctuation, so `# short` and
  `# Short` are the same. `sentance` is accepted for `Sentence`. The older,
  longer names (`One sentence`, `Three paragraphs`, `One page`, `Three pages`)
  no longer work, and neither does `##` as a depth marker.
* Everything below a heading is normal markdown.

The reader's depth is kept between posts, in `localStorage` under `depth`. A
post that does not carry the chosen depth opens at the nearest one it has,
and leaves the choice alone; only moving the slider sets a new one.

The server rereads `posts/` when a file changes, so you can edit a post and
refresh the page. When it runs from the project folder, it also live reloads
`templates/*.html` and `static/*.css`: template edits refresh the browser and
stylesheet edits appear without a page refresh. Static exports still use the
files embedded in the binary.

`audit.sh` gives one row per file — the words in each of the five depths, the
total written, and the time to read the longest depth — and under a row,
whatever is wrong with that file.

    make audit                  # the table and the findings
    make stats                  # the table alone
    scripts/audit.sh --words    # the most words first
    scripts/audit.sh posts/a.md # one file

The **title carries the verdict**: plain when the file is clean, amber when it
holds a warning, red when it holds an error. It reports:

* the depth headings a file does not carry, and a heading with nothing under it
* **a heading of one `#` that names no depth.** The site matches every `#`
  after the title against the five names; one that matches nothing takes its
  text with it, and those words never reach the page. This is an error.
* a missing title, or a missing `date` or `tag`
* a `Sentence` depth above 35 words, or a `Paragraph` depth at 120 or more
* a picture the file points at that is not in `posts/images/`
* a word written twice in a row, where the two words touch

Rows come in the order of the index: pinned first, then the newest. A depth
that is not written shows a dot. The state — ` draft`, ` pinned` — sits at
the end of the row on purpose: a mark from a nerd font may take two columns
while it counts as one character, so nothing that can change width is put to
the left of a number. Every cell is padded by character, not by byte, because
`printf` counts bytes and a dot is two of them.

It counts the way the site counts: a picture and its caption are not prose, and
a code fence does count. It ends with 1 if any file holds an error, and 0 for
warnings only, so it fits in a hook or a pipeline. A missing depth is a
warning, because a post may stop early by design, and a `draft: true` file may
hold nothing at all.

The report is coloured, and a pipe turns the colour off. Two switches change
that:

    AUDIT_COLOR=always|never   force the colour on or off
    AUDIT_ICONS=0              plain marks, for a terminal with no nerd font

## draft and pinned

Two front matter properties change where a file goes.

    ---
    date: 2026-08-14
    tag: materials
    draft: true      # on the index while you run it, never in the build
    pinned: true     # first on the index, and a page, not a post
    color: emerald   # this page's accent
    depth: short     # the depth it opens at
    ---

`true`, `yes`, `on` and `1` all mean yes, in any case. Anything else means no.

**draft** — the post is on the index and at its own URL while `make run` is up,
with a **draft** mark beside it, so you read it as a reader would. `make export`
leaves it out: no page, and no line on the index.

**pinned** — the page sits above every post on the index, whatever its date,
and it drops the furniture of a post: no tag, no date, no depth count. It shows
the title and the reading time only, and its own page carries no meta line. It
keeps the depth slider if it holds more than one depth. Two pinned pages keep
the newest first.

## color

A post may take its own accent with `color:` in the front matter. It changes
the tag, the depth label, the slider, a link in the prose, and every hover on
that page. The index and the other posts keep the site's own accent.

    ---
    color: emerald
    ---

The names are the ones Tailwind uses, because they are easy to remember:

`slate` · `gray` · `zinc` · `neutral` · `stone` · `red` · `orange` · `amber` · `yellow` · `lime` · `green` · `emerald` · `teal` · `cyan` · `sky` · `blue` · `indigo` · `violet` · `purple` · `fuchsia` · `pink` · `rose`

The values are not Tailwind's. Each one was found by walking the lightness of
its hue until it reads at about **5.6:1** on the paper and **8.4:1** on the
dark ground, which is where the site's own accent sits. An accent is a link
colour here, so it has to be readable, not only visible. Every name carries
two values, one for each ground, and the theme picks.

A name that is not on the list is dropped: the page keeps the site's accent,
and `make audit` reports the line. To change a value, or to add a name, the
whole palette is the block under `the named colours` in `static/style.css`.

## The build of the site

    make export             # into ./dist
    make export DIST=out    # somewhere else

It writes the URLs the server already answers on, so nothing changes when you
deploy:

    dist/index.html
    dist/post/<slug>/index.html
    dist/404.html
    dist/static/style.css   app.js
    dist/media/*

A draft never reaches it, and neither does a file with no depth. `make export`
empties the folder first, so a post you renamed leaves no page behind; the Go
code itself deletes nothing, and the `rm` sits on the line that does it.

The finished program does the same build, without the repository:

    ./build/baumkuchen -posts ~/blog -export out

### The folder the site is published in

Every link the site writes starts at the root, because that is where the server
answers. A GitHub **project page** is served from a folder — this one is at
`/baumkuchen/` — so the build puts that folder in front of each link:

    BASE ?= /baumkuchen

Leave `BASE` empty for a custom domain or a user site, both of which are served
from the root. Only `href="/…"` and `src="/…"` are touched, so an address with
a scheme, or one that starts `//`, is left alone. The running server ignores
`BASE` altogether: `make run` always answers at the root.

## depth

A post may open at a depth of its own with `depth:` in the front matter. It
takes the same five names the headings take.

    ---
    depth: short
    ---

Three things can say which depth a post opens at, and they rank:

1. **the address** — `#medium` in a link the reader followed
2. **the post** — this `depth:` line
3. **the reader** — the depth they last chose, kept in `localStorage`

So a post that is worth meeting at its Short version says so, and that beats
the reader's standing choice; but a link someone sent them still wins, because
that link was made on purpose. Moving the slider still writes the reader's
choice, so the next post they open follows them again.

A name that is not one of the five is dropped, and the post opens the way every
other post does. `make audit` reports that, and also a `depth:` naming a depth
the post does not carry — the post still opens at the nearest one it has.

## Moving between the depths

Four ways, all the same move:

* the **slider**, dragged or clicked anywhere along it
* the **‹ and ›** buttons on either side of it, one depth at a time
* the **left and right arrow keys**, from anywhere on the page
* a **link** that ends in a depth name

Up, down, home and end work as well, but only while the slider itself has the
focus: those keys scroll the page, and taking them from a reader who is only
reading would be rude. The arrows are safe to take, because nothing on the
page scrolls sideways. Typing in a field is never interrupted.

## A link to a depth

The address carries the depth: `/post/why-rivers-meander#medium`. The five
names are the ones in the markdown — `#sentence`, `#paragraph`, `#short`,
`#medium`, `#long`.

Moving the slider writes the name into the address with `replaceState`, so the
page does not jump and the back button leaves the post instead of stepping
through every depth. A copied link opens at that depth; a post that does not
carry it opens at the nearest one it has. The address beats the depth kept in
`localStorage`. A hash that is not one of the five is left to the browser, so a
footnote link still works.

Only the depth on screen is in the page for a reader view or a clipper: the
others carry `aria-hidden` and `inert`, which is what those tools read. A
clipping holds one depth, not five.

## The scripts

    scripts/new-draft.sh  ## a new draft, with the skeleton written for you
    scripts/audit.sh      ## what is written, and what is wrong with it

`new-draft.sh` asks for the title, or takes it as an argument. It builds the
file name from the title (lower case, hyphens), refuses to write over a file
that is already there, and prints the path it wrote. The file lands in
`posts/posts/` and carries every setting a post can take, so nothing has to be
remembered:

    ---
    date: 2026-08-20
    tag:
    draft: true
    pinned: false
    color:
    depth:
    ---

An empty setting does nothing, so a file can be left as it is written. Give
`INTO` to write somewhere else: `INTO=_drafts scripts/new-draft.sh "…"`.

`audit.sh` reads every file in `posts/` and reports:

* the depth headings a file does not carry, and a heading with nothing under it
* **a heading of one `#` that names no depth.** The site matches every `#`
  after the title against the five names; one that matches nothing takes its
  text with it, and those words never reach the page. This is an error.
* a missing title, or a missing `date` or `tag`
* a `Sentence` depth above 35 words, or a `Paragraph` depth at 120 or more
* a picture the file points at that is not in `posts/images/`
* a word written twice in a row, where the two words touch

Beside each file name it shows what the file is:  **draft** (out of the
build) and  **pinned** (top of the index).

It counts words the way the site counts them: a picture and its caption are not
prose, and the words inside a code fence do count. It ends with 1 if any file
holds an error, and 0 for warnings only, so it fits in a hook or a pipeline. A
missing depth is a warning, because a post may stop early by design, and a
`draft: true` file may hold nothing at all.

The report is coloured, with nerd font icons. A pipe turns the colour off.
Two switches change that:

    AUDIT_COLOR=always|never   force the colour on or off
    AUDIT_ICONS=0              plain marks, for a terminal with no nerd font

## Files

    main.go     the command: the flags, the settings, the two ways to run
    internal/blog/  the site: the posts, the pages, the build, the server
    internal/content/  where the posts come from: a folder, or files in the program
    internal/bundle/   putting the posts inside a copy of the program
    templates/  layout, index, post, 404
    static/     one stylesheet, one script
    posts/      the posts, which live outside this repository
    posts/images/  pictures, served at /media/
    test-blog/  a second set of posts, made for the test

The tests run on posts made for them. The one test that reads the real posts
skips when they are not there, and runs when `BAUMKUCHEN_POSTS` names them.

`test-blog/` holds ten articles about basic science, each written at all five
depths and each with its own pictures. They are written for the test, and they
are read like any other folder of posts:

    ./build/baumkuchen -posts test-blog -addr 127.0.0.1:8080
    POSTS=test-blog IMAGES=test-blog/images scripts/audit.sh

## The checks

    make check   # the format and the vet; changes no file
    make test    # the tests
    make lint    # repair the format, then report what is left
    make smoke   # the finished program, out of the repository, doing its work

The same checks run on every push, in `.github/workflows/ci.yml`.

