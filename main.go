// Baumkuchen is a blog that serves markdown from a folder, at up to five
// lengths.
//
//	baumkuchen                  # http://localhost:8080
//	baumkuchen -addr :3000
//	baumkuchen -export dist     # write the static site, then stop
//
// The settings live in baumkuchen.yml; a flag beats the file, and the file
// beats the defaults. Everything else is in internal/blog. This file holds
// the two embedded folders, because //go:embed reads its paths from the
// folder of the package that asks for them.
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
	conf := flag.String("config", "baumkuchen.yml", "the settings file")
	addr := flag.String("addr", "", "listen address")
	dir := flag.String("posts", "", "folder with markdown posts")
	out := flag.String("export", "", "write the static site to this folder, then stop")
	base := flag.String("base", "", "the folder the built site is published in, as in /baumkuchen")
	title := flag.String("title", "", "the name of the site")
	flag.Parse()

	cfg, found, err := blog.LoadFile(*conf)
	if err != nil {
		log.Fatal(err)
	}
	// a flag beats the file, but only a flag that was actually given
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "addr":
			cfg.Addr = *addr
		case "posts":
			cfg.Posts = *dir
		case "base":
			cfg.Base = *base
		case "title":
			cfg.Title = *title
		}
	})

	site, err := blog.New(blog.Config{
		Posts: cfg.Posts, Templates: templates, Static: static,
		Base: cfg.Base, Title: cfg.Title,
	})
	if err != nil {
		log.Fatal(err)
	}
	if !found {
		log.Printf("no %s, so the defaults stand", *conf)
	}
	for _, p := range site.Incomplete() {
		log.Printf("no depth %-28s missing: %s", p.Slug, strings.Join(p.Missing, ", "))
	}
	published, drafts, incomplete := site.Counts()
	log.Printf("%d published, %d draft(s), %d without a depth in %s",
		published, drafts, incomplete, cfg.Posts)

	// the build: write the files and stop, with no draft among them
	if dest := export(*out, cfg.Dist); dest != "" {
		n, err := site.Export(dest)
		if err != nil {
			log.Fatalf("export: %v", err)
		}
		log.Printf("%d file(s) written to %s", n, dest)
		return
	}

	log.Printf("blog on http://localhost%s", cfg.Addr)
	if err := site.Serve(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}

// export gives the folder to build into: the flag's own value, or the one in
// the settings when the flag was given with nothing after it.
func export(flagValue, fromFile string) string {
	if flagValue == "" {
		return ""
	}
	if flagValue == "-" {
		return fromFile
	}
	return flagValue
}
