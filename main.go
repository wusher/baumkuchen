// Baumkuchen is a blog that serves markdown from a folder, at up to five
// lengths.
//
//	baumkuchen                  # http://localhost:8080
//	baumkuchen -addr :3000
//	baumkuchen -export dist     # write the static site, then stop
//
// Everything else is in internal/blog. This file holds the flags and the two
// embedded folders, because //go:embed reads its paths from the folder of the
// package that asks for them.
package main

import (
	"embed"
	"flag"
	"log"
	"strings"

	"baumkuchen/internal/blog"
)

//go:embed templates/*.html
var templates embed.FS

//go:embed static/*
var static embed.FS

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dir := flag.String("posts", "posts", "folder with markdown posts")
	out := flag.String("export", "", "write the static site to this folder, then stop")
	flag.Parse()

	site, err := blog.New(blog.Config{Posts: *dir, Templates: templates, Static: static})
	if err != nil {
		log.Fatal(err)
	}
	for _, p := range site.Incomplete() {
		log.Printf("no depth %-28s missing: %s", p.Slug, strings.Join(p.Missing, ", "))
	}
	published, drafts, incomplete := site.Counts()
	log.Printf("%d published, %d draft(s), %d without a depth in %s",
		published, drafts, incomplete, *dir)

	// the build: write the files and stop, with no draft among them
	if *out != "" {
		n, err := site.Export(*out)
		if err != nil {
			log.Fatalf("export: %v", err)
		}
		log.Printf("%d file(s) written to %s", n, *out)
		return
	}

	log.Printf("blog on http://localhost%s", *addr)
	if err := site.Serve(*addr); err != nil {
		log.Fatal(err)
	}
}
