---
date: 2026-11-10
tag: notes
pinned: true
color: violet
---

# Colophon

# Sentence

This blog is made by a small Go program. It reads the markdown, cuts each file into five depths, writes a plain static site, and GitHub Pages publishes the result.

# Paragraph

This blog is made by a small Go program. It reads a folder of markdown, cuts each file into the five depths the site knows, and writes a plain static site: an index, a page for each article, a stylesheet and a script. The test copy lives in the repository, so a workflow builds it on every change and publishes it through GitHub Pages. Nothing here needs a database, a server or an account.

# Short

This blog is made by a small Go program, roughly a few thousand lines, with the templates and the stylesheet carried inside it. It reads a folder of markdown files and writes a plain static site: an index, one page for each article, a not-found page, a stylesheet and a script. There is no database, no server and no account.

The writing for the test lives in the repository, in the folder called `test-blog`, so it can be built anywhere. The real posts live outside the repository, behind a link that only exists on the machine that writes them, which is why the workflow builds the test copy instead.

The build is one command, and it is checked in two ways: a workflow that formats, vets, tests and runs the finished program on its own, and a second workflow that builds the test blog and publishes it to GitHub Pages. Both run on their own when a change lands on the main branch. Nothing here was dispatched by hand.

# Medium

The site is deliberately small. Here is the whole of it.

## The program

The program is written in Go and lives in one repository. It reads markdown files, works out which depth each heading names, renders them, and writes the result out as files. The depth logic is the heart of it: a file is cut into the five depths by its headings, and the front matter decides where the result sits on the index.

Templates and stylesheet are carried inside the binary, so the finished program can be copied anywhere and will still build a whole site. The markdown can also be carried inside it, which makes a single file that holds both the program and the writing.

## The writing

Two sets of posts exist. The real ones live outside the repository, so they are not in the build, and the link that points at them only exists on the machine that writes them. The test ones — the ten articles and the three pages you can see here — live in the repository in the folder `test-blog`, so they are built by the workflow on every change.

The same program reads either folder. The folder is named on the command line, and the audit reads whichever one it is pointed at, which is why the test copy is held to the same rules as the real one.

## The build and the publishing

A check workflow runs the format check, the vet, the tests, the finished binary and a smoke test that copies the binary out of the repository and runs it there with the sources nowhere near it.

A second workflow builds the test blog into a folder of plain files and publishes that folder to GitHub Pages. It refuses to publish an empty build, and it checks that every link in the built site carries the folder the site is served from, so the pages do not break once they are online. Both workflows start by themselves; neither is ever run by hand.

# Long

A colophon is a note about how something was made, so here is the whole account.

## The program

The site is a single Go program. It takes a folder of markdown and gives back a folder of files that any static host can serve: an index, one page for each article, a not-found page, a stylesheet, a script, and the pictures.

The work happens in three steps. First the program reads every markdown file it finds and works out what is in it. Each file is cut into depths by its headings: a heading of one hash marks the title and then names a depth, and a heading of two or more is ordinary text inside the depth it sits in. The front matter on top of the file carries the date, the tag, and the flags that decide where the file lands.

Then the templates are rendered. There are three of them — the index, an article and the not-found page — and they are carried inside the binary, so the program does not need the repository to build a site. Finally the pictures are copied and shrunk on the way out.

The finished program stands on its own. It can be copied to a machine with no Go on it, no repository and no network, and it will still read a folder of markdown and write a working site.

## The writing

The real posts live on the machine that writes them, outside the repository, and the repository holds a link that points at them. That link is useless anywhere else, so the repository also holds a second, separate set: the test blog, in the folder `test-blog`, with ten articles and three pages.

The two are read the same way. The folder is named on the command line, so the program does not care which one it is given, and the audit reads whichever folder it is pointed at. The test blog is therefore held to the same rules as the real writing: a sentence of at most thirty-five words, a paragraph of fewer than a hundred and twenty, every depth present, every picture present, no word written twice in a row.

## The two workflows

One workflow checks the program. It runs the format check and the vet, then the tests, then builds the binary and runs a smoke test that copies the binary out of the repository and runs it there with the sources nowhere near it. It runs on every push to the main branch and on every branch under the same owner.

The second workflow publishes the test blog. It builds the site into a folder of plain files, refuses to go on if the build is missing or empty, checks that the number of pages in the build matches the number of posts the program says it published, and checks that every root-absolute link in the built site carries the folder the site is served from. Then it hands the folder to GitHub Pages.

Both workflows start by themselves when a change lands on the main branch. Neither is ever dispatched by hand, and the settings of neither is touched to make a change appear.

## The tools

The templates are plain Go templates, the styling is plain CSS with custom properties for the colours and the dark theme, and the markdown is rendered by a well-known library. There is no framework in front of any of it, and no build step that is not a Go command.
