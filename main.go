// Baumkuchen is a blog that serves markdown from a folder, at up to five
// lengths.
//
//	baumkuchen                          # serve the posts, live, on the address in the settings
//	baumkuchen ~/blog                  # serve the posts in another folder
//	baumkuchen -posts ~/blog -export out   # write the static site, then stop
//
// The program is one file. The templates, the stylesheet, the script and the
// settings are built into it, so a copy of it runs anywhere, without the
// folder it was made in.
//
// The posts are not built in. They come from a folder you name, so a site is
// never a copy of somebody else's writing by accident. A copy of the program
// with the posts inside is made on purpose: see `make bundle`.
//
// Everything else is in internal/blog. This file holds the two embedded
// folders, because //go:embed reads its paths from the folder of the package
// that asks for them.
package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"

	"baumkuchen/internal/blog"
	"baumkuchen/internal/bundle"
	"baumkuchen/internal/content"
)

//go:embed templates/*.html
var templates embed.FS

//go:embed static/*
var static embed.FS

//go:embed baumkuchen.yml
var settings []byte

// The build puts its own name in here, so a copy of the program can say which
// one it is.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	conf := flag.String("config", "baumkuchen.yml", "the settings file")
	addr := flag.String("addr", "", "listen address")
	dir := flag.String("posts", "", "folder with the markdown posts")
	out := flag.String("export", "", "write the static site to this folder, then stop")
	base := flag.String("base", "", "the folder the built site is published in, as in /baumkuchen")
	title := flag.String("title", "", "the name of the site")
	showVersion := flag.Bool("version", false, "print the version and stop")
	pack := flag.String("bundle", "", "write a copy of this program with the content in this folder inside it")
	to := flag.String("o", "", "where to write that copy")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] [posts-directory]\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	postsDir, postsGiven, err := postsArgument(flag.Args(), *dir, given("posts"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		flag.Usage()
		os.Exit(2)
	}

	if *showVersion {
		fmt.Println(versionLine())
		return
	}
	if *pack != "" {
		if err := makeBundle(*pack, *to); err != nil {
			log.Fatal(err)
		}
		return
	}

	cfg, from, err := loadSettings(*conf, given("config"))
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

	if postsGiven {
		cfg.Posts = postsDir
	}
	src, err := chooseSource(cfg, postsGiven)
	if err != nil {
		log.Fatal(err)
	}

	dest := export(*out, cfg.Dist)
	templateFiles, staticFiles := fs.FS(templates), fs.FS(static)
	liveReload := false
	if dest == "" {
		if _, templateErr := os.Stat("templates/layout.html"); templateErr == nil {
			if _, staticErr := os.Stat("static/style.css"); staticErr == nil {
				templateFiles, staticFiles = os.DirFS("."), os.DirFS(".")
				liveReload = true
			}
		}
	}
	site, err := blog.New(blog.Config{
		Content: src, Templates: templateFiles, Static: staticFiles,
		Base: cfg.Base, Title: cfg.Title, LiveReload: liveReload,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("the settings come from %s", from)
	log.Printf("the posts come from %s", src.Label())
	for _, p := range site.Incomplete() {
		log.Printf("no depth %-28s missing: %s", p.Slug, strings.Join(p.Missing, ", "))
	}
	published, drafts, incomplete := site.Counts()
	log.Printf("%d published, %d draft(s), %d without a depth",
		published, drafts, incomplete)

	// the build: write the files and stop, with no draft among them
	if dest != "" {
		n, err := site.Export(dest)
		if err != nil {
			log.Fatalf("export: %v", err)
		}
		log.Printf("%d file(s) written to %s", n, dest)
		return
	}

	if liveReload {
		log.Printf("live reload watching templates/*.html and static/*.css")
	}
	log.Printf("the blog is on http://%s", host(cfg.Addr))
	if err := site.Serve(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}

// postsArgument accepts one positional folder as shorthand for -posts.
func postsArgument(args []string, dir string, explicit bool) (string, bool, error) {
	if len(args) > 1 {
		return "", false, errors.New("expected at most one posts directory; put flags before the directory")
	}
	if len(args) == 1 {
		if explicit {
			return "", false, errors.New("use either -posts or a positional posts directory, not both")
		}
		return args[0], true, nil
	}
	return dir, explicit, nil
}

// host gives the address to show in the log: a port on its own means this
// machine.
func host(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	return addr
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

// loadSettings reads the settings file.
//
// When the file is not there, and it was not named on the command line, the
// settings inside the program stand: a copy of the program on its own still
// knows the site. A file that was named and is not there is a mistake, and it
// is reported as one.
func loadSettings(path string, wasGiven bool) (blog.File, string, error) {
	_, err := os.Stat(path)
	if err == nil {
		cfg, _, err := blog.LoadFile(path)
		if err != nil {
			return blog.File{}, "", err
		}
		return cfg, path, nil
	}
	if wasGiven {
		return blog.File{}, "", fmt.Errorf("%s: %w", path, err)
	}
	cfg, err := blog.Load(settings)
	if err != nil {
		return blog.File{}, "", err
	}
	return cfg, "the settings inside the program", nil
}

// chooseSource says where the posts come from.
//
// The folder named on the command line comes first, then the one in the
// environment, then the one in the settings. When none of them is there, the
// posts carried inside the program are used, if it carries any. A program
// with no posts at all says so, and names the ways to give it some.
func chooseSource(cfg blog.File, postsGiven bool) (content.Source, error) {
	inside, err := bundledContent()
	if err != nil {
		return nil, err
	}
	if postsGiven {
		src := content.NewDir(cfg.Posts)
		if !src.Exists() {
			return nil, fmt.Errorf("no folder at %s", cfg.Posts)
		}
		return src, nil
	}
	if env := os.Getenv("BAUMKUCHEN_POSTS"); env != "" {
		src := content.NewDir(env)
		if !src.Exists() {
			return nil, fmt.Errorf("BAUMKUCHEN_POSTS: no folder at %s", env)
		}
		return src, nil
	}
	if cfg.Posts != "" {
		src := content.NewDir(cfg.Posts)
		if src.Exists() {
			return src, nil
		}
		if inside != nil {
			return inside, nil
		}
		return nil, fmt.Errorf("no folder at %s: name one with -posts, or set BAUMKUCHEN_POSTS", cfg.Posts)
	}
	if inside != nil {
		return inside, nil
	}
	return nil, errors.New("no posts: name a folder with -posts FOLDER, or set BAUMKUCHEN_POSTS")
}

// bundledContent gives the posts carried inside this program, if it carries
// any. A program without them is not a fault.
func bundledContent() (content.Source, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, nil
	}
	files, ok, err := bundle.Open(exe)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return content.NewFS(files, bundle.Root), nil
}

// makeBundle writes a copy of this program with the content inside it. The
// folder is named on purpose: nothing is ever put inside a program by
// accident.
func makeBundle(dir, out string) error {
	if out == "" {
		return errors.New("bundle: say where to write it: -o FILE")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	n, err := bundle.Write(exe, out, dir)
	if err != nil {
		return err
	}
	log.Printf("%d file(s) from %s are inside %s", n, dir, out)
	return nil
}

// given says whether a flag was actually given.
func given(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

// versionLine gives the name of the build.
func versionLine() string {
	return fmt.Sprintf("baumkuchen %s (%s, %s)", version, commit, date)
}
