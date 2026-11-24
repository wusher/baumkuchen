package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/wusher/baumkuchen/internal/blog"
	"github.com/wusher/baumkuchen/internal/bundle"
	"github.com/wusher/baumkuchen/internal/content"
)

// errQuiet is an error the command has already said out loud. The run ends
// with the first code, and nothing more is printed.
var errQuiet = errors.New("quiet")

// work is one run of the program: what was asked for, and where to say it.
type work struct {
	out io.Writer // where the result goes
	err io.Writer // where the log goes
	cmd *commandLine
	// set is the settings of the run, once they have been read.
	set *settings
	// bundled is the content carried inside the program, when it carries any.
	bundled fs.FS
}

// dispatch reads the command line and does what it asks.
func dispatch(argv []string, stdout, stderr io.Writer) error {
	cmd, err := readArgs(argv)
	if err != nil {
		return err
	}
	w := &work{out: stdout, err: stderr, cmd: cmd}
	w.setupLog()
	if cmd.name == "help" {
		return w.help()
	}
	if cmd.globals.help {
		fmt.Fprint(stdout, usageText(cmd.name, cmd.named))
		return nil
	}
	// the version is asked for with a flag as well as with a command, so a
	// script can ask for it without knowing the commands
	if cmd.globals.version {
		return w.version()
	}
	c, ok := commandByName(cmd.name)
	if !ok {
		return usagef("unknown command %s", cmd.name)
	}
	return c.run(w)
}

// help shows the help: the whole program, or the help of one command.
//
// The help is a command as well as a flag, so it can be asked for on its own:
// baumkuchen help, and baumkuchen help build.
func (w *work) help() error {
	if w.cmd.subject != "" {
		fmt.Fprint(w.out, commandUsage(w.cmd.subject))
		return nil
	}
	fmt.Fprint(w.out, programUsage())
	return nil
}

// setupLog sends the log to the error stream, where it belongs.
//
// The result of a command goes to the standard output, so a pipe gets the
// result and nothing else. The quiet flag says nothing at all but the result.
func (w *work) setupLog() {
	log.SetOutput(w.err)
	log.SetFlags(0)
	if w.cmd.globals.quiet {
		log.SetOutput(io.Discard)
	}
}

// settings reads the settings for the run, with the flags on top of them.
func (w *work) settings() (*settings, error) {
	if w.set != nil {
		return w.set, nil
	}
	bundled, err := w.bundledContent()
	if err != nil {
		return nil, err
	}
	set, err := findSettings(w.cmd.globals.config, w.cmd.folder, bundled)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"title", "heading", "favicon", "base"} {
		if v, ok := w.cmd.value(name); ok {
			set.file.Set(name, v)
		}
	}
	w.set = set
	return set, nil
}

// bundledContent gives the content carried inside this program, if it carries
// any. A program without them is not a fault.
func (w *work) bundledContent() (fs.FS, error) {
	if w.bundled != nil {
		return w.bundled, nil
	}
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
	w.bundled = files
	return files, nil
}

// postsFolder gives the folder the posts are read from.
//
// The folder named on the command line comes first, then the one in the
// environment, then the one in the settings. When none of them is there, the
// posts carried inside the program are used, if it carries any.
//
// A folder named on the command line that holds a settings file of its own
// is read through that file: the posts line in it says where the writing
// is, and that is where the writing is read from.
//
// The settings inside the program do not name a folder on the disk of the
// machine the program runs on: a path in them is a path inside the program.
// The posts are the ones it carries, and not the posts folder of the folder
// it happens to run in.
func (w *work) postsFolder(set *settings) (folder string, inside bool, err error) {
	if w.cmd.folder != "" {
		if !content.NewDir(w.cmd.folder).Exists() {
			return "", false, fmt.Errorf("no folder at %s", w.cmd.folder)
		}
		if set != nil && set.names("posts") && set.from != "" &&
			sameFolder(set.from, w.cmd.folder) {
			folder = set.resolve(set.file.Posts)
			if content.NewDir(folder).Exists() {
				return folder, false, nil
			}
		}
		return w.cmd.folder, false, nil
	}
	if env := os.Getenv("BAUMKUCHEN_POSTS"); env != "" {
		if !content.NewDir(env).Exists() {
			return "", false, fmt.Errorf("BAUMKUCHEN_POSTS: no folder at %s", env)
		}
		return env, false, nil
	}
	if set.inside {
		return "", true, nil
	}
	if set.file.Posts != "" {
		folder = set.resolve(set.file.Posts)
		if content.NewDir(folder).Exists() {
			return folder, false, nil
		}
	}
	return "", true, nil
}

// sameFolder says whether a path is in a folder: the settings file of the
// folder that was named is the one that speaks for it.
func sameFolder(path, folder string) bool {
	dir := filepath.Dir(path)
	if dir == folder {
		return true
	}
	a, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	b, err := filepath.Abs(folder)
	if err != nil {
		return false
	}
	return a == b
}

// source says where the posts come from.
func (w *work) source(set *settings) (content.Source, error) {
	folder, inside, err := w.postsFolder(set)
	if err != nil {
		return nil, err
	}
	if !inside {
		return content.NewDir(folder), nil
	}
	files, err := w.bundledContent()
	if err != nil {
		return nil, err
	}
	if files == nil {
		return nil, errors.New(
			"no posts: name a folder with `baumkuchen serve FOLDER`, or set BAUMKUCHEN_POSTS")
	}
	return content.NewFS(files, bundle.Root), nil
}

// site makes the site a command works on: the settings, the posts, and the
// files the pages are made from.
//
// The running server watches the writing, so that an edited post refreshes
// the browser by itself. The --dev flag adds the files on disk and the errors
// in full.
func (w *work) site(drafts bool) (*blog.Site, error) {
	set, err := w.settings()
	if err != nil {
		return nil, err
	}
	src, err := w.source(set)
	if err != nil {
		return nil, err
	}
	templateFS, staticFS, err := w.files()
	if err != nil {
		return nil, err
	}
	log.Printf("the settings come from %s", set.from)
	log.Printf("the posts come from %s", src.Label())
	return blog.New(blog.Config{
		Content:       src,
		Templates:     templateFS,
		Static:        staticFS,
		Base:          set.file.Base,
		Title:         set.file.Title,
		Heading:       set.file.Heading,
		Favicon:       set.file.Favicon,
		FaviconInside: set.inside,
		LiveReload:    w.watching(),
		DevFiles:      w.dev(),
		HideDrafts:    !drafts,
		DevErrors:     w.dev(),
	})
}

// files gives the templates and the stylesheet the pages are made from.
//
// They are the ones built into the program, so a copy of it stands on its
// own. The --dev flag asks for the ones on disk instead, so that a change to
// a template or to the stylesheet shows up without a build.
//
// The program does not guess: a folder that happens to hold a templates
// folder is not a reason to read from disk. The flag is asked for, or the
// templates come from inside the program.
func (w *work) files() (templates, static fs.FS, err error) {
	if w.dev() {
		if _, err := os.Stat(filepath.Join("templates", "layout.html")); err != nil {
			return nil, nil, fmt.Errorf("--dev: there are no templates/layout.html here")
		}
		return os.DirFS("."), os.DirFS("."), nil
	}
	return fs.FS(templateFiles), fs.FS(staticFiles), nil
}

// watching says whether an open browser is told when the writing changes.
//
// The running server watches, so that an edited post refreshes the page
// itself. The build does not watch, and writes no watcher into the pages it
// writes.
func (w *work) watching() bool { return w.cmd.name == "serve" }

// dev says whether the development mode was asked for.
func (w *work) dev() bool { return w.cmd.was("dev") }

// addr gives the address the server listens on: the flag, then the settings,
// then the default in the code.
func (w *work) addr(set *settings) string {
	if v, ok := w.cmd.value("addr"); ok {
		return v
	}
	if set.file.Addr != "" {
		return set.file.Addr
	}
	return blog.Defaults().Addr
}

// outFolder gives the folder the built site is written into: the flag, then
// the settings, then the default in the code.
func (w *work) outFolder(set *settings) string {
	if v, ok := w.cmd.value("out"); ok {
		return v
	}
	if set.file.Dist != "" {
		return set.resolve(set.file.Dist)
	}
	return blog.Defaults().Dist
}

// serve answers on an address until it cannot.
func (w *work) serve() error {
	set, err := w.settings()
	if err != nil {
		return err
	}
	addr := w.addr(set)
	// the address is looked at before the writing is read, so a mistake in it
	// is the first thing the writer hears about
	if err := checkAddr(addr); err != nil {
		return err
	}
	drafts := w.drafts(addr)
	site, err := w.site(drafts)
	if err != nil {
		return err
	}
	published, draftCount, incomplete := site.Counts()
	for _, p := range site.Incomplete() {
		log.Printf("no depth %-28s missing: %s", p.Slug, strings.Join(p.Missing, ", "))
	}
	log.Printf("%d published, %d draft(s), %d without a depth",
		published, draftCount, incomplete)

	// the socket is opened before anything is said, so the address that is
	// printed is the address the site answers on
	ln, err := site.Listen(addr)
	if err != nil {
		return listenError(addr, err)
	}
	defer ln.Close()
	url := siteURL(ln.Addr())
	fmt.Fprintf(w.out, "the blog is on %s\n", url)
	if w.cmd.on("open") {
		if err := openInBrowser(url); err != nil {
			log.Printf("open a browser: %v", err)
		}
	}
	return site.ServeOn(ln)
}

// checkAddr says whether the address can be listened on.
//
// An address without a port in it is a mistake the writer can fix in one
// step, so it is named, and the two forms that work are given. A port that
// is not a number is the same mistake in the other half of the address.
//
// The check is made before the socket is opened, so the message comes
// before anything else is said about the site.
func checkAddr(addr string) error {
	_, port, err := net.SplitHostPort(addr)
	if err == nil {
		if _, perr := strconv.Atoi(port); perr != nil {
			return usagef("the port in %s is not a number: use :8080 or 127.0.0.1:8080", addr)
		}
		return nil
	}
	if isNumber(addr) {
		return usagef("the address %s has no port in it: use :%s or 127.0.0.1:%s",
			addr, addr, addr)
	}
	return usagef("the address %s has no port in it: use :8080 or 127.0.0.1:8080", addr)
}

// isNumber says whether a word is a number.
func isNumber(word string) bool {
	_, err := strconv.Atoi(word)
	return err == nil
}

// drafts says whether the drafts are shown.
//
// They are shown on the loopback address, where only this machine answers, and
// they are held back anywhere else, where anyone can. The --drafts and
// --no-drafts flags say so either way.
func (w *work) drafts(addr string) bool {
	if w.cmd.was("drafts") {
		return true
	}
	if w.cmd.was("no-drafts") {
		return false
	}
	return loopback(addr)
}

// loopback says whether an address is one only this machine answers on.
func loopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.ToLower(strings.Trim(host, "[]"))
	switch host {
	case "", "0.0.0.0", "::":
		return false
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// siteURL gives the address to show, in the form a browser can open.
func siteURL(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "http://" + addr.String()
	}
	switch host {
	case "", "0.0.0.0", "::":
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}

// listenError says the address could not be opened.
//
// The wording of the operating system is not the same everywhere: on Linux it
// says "address already in use", and on Windows it says "only one usage of
// each socket address". Both mean that the port is taken.
func listenError(addr string, err error) error {
	if !addrInUse(err) {
		return err
	}
	return fmt.Errorf("%s is already taken: %w", addr, err)
}

// addrInUse says whether the error is the one that means the port is taken.
func addrInUse(err error) bool {
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "address already in use") ||
		strings.Contains(text, "only one usage of each socket address")
}

// openInBrowser opens a page in a browser, when there is one to open it.
func openInBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// build writes the static site.
//
// The output folder and the writing folder are checked against each other
// before anything is written or removed: the site and the writing are
// different things, and a build must not put one inside the other.
func (w *work) build() error {
	set, err := w.settings()
	if err != nil {
		return err
	}
	out := w.outFolder(set)
	posts, inside, err := w.postsFolder(set)
	if err != nil {
		return err
	}
	if !inside {
		if err := checkBuildPaths(out, posts); err != nil {
			return err
		}
	}
	drafts := w.cmd.was("drafts")
	site, err := w.site(drafts)
	if err != nil {
		return err
	}
	if w.cmd.was("clean") {
		if err := blog.CleanBuild(out); err != nil {
			return err
		}
	}
	n, err := site.ExportWith(out, drafts)
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}
	published, draftCount, incomplete := site.Counts()
	fmt.Fprintf(w.out, "%d published post(s), %d draft(s), %d without a depth\n",
		published, draftCount, incomplete)
	fmt.Fprintf(w.out, "%d file(s) written to %s\n", n, out)
	return nil
}

// bundle writes one file with the program and the posts inside it.
func (w *work) bundle() error {
	out, _ := w.cmd.value("out")
	if out == "" {
		return usagef("bundle: say where to write it: --out FILE")
	}
	set, err := w.settings()
	if err != nil {
		return err
	}
	folder, cleanup, err := w.bundleFolder(set)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := hasMarkdown(folder); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	extra := map[string][]byte{bundle.Root + "/" + settingsName: set.bytes}
	// the picture on the tab travels inside the program as well
	icon, iconName, err := bundleIcon(set)
	if err != nil {
		return err
	}
	if iconName != "" {
		extra[bundle.Root+"/"+iconName] = icon
		raw, err := blog.WithFavicon(set.bytes, iconName)
		if err != nil {
			return err
		}
		extra[bundle.Root+"/"+settingsName] = raw
	}
	n, err := bundle.WriteWith(exe, out, folder, extra)
	if err != nil {
		return err
	}
	fmt.Fprintf(w.out, "%d file(s) from %s are inside %s\n", n, folder, out)
	return nil
}

// bundleIcon gives the picture on the tab, and the name it has inside the
// program.
//
// The settings name a file on the disk of the writer. A bundle travels, so
// the picture travels with it: it is put inside, and the settings that go
// inside are changed to name the picture that is there.
//
// The file is looked for the way the program looks for it — a relative path
// starts in the working folder — and then beside the settings file.
//
// A picture that is not there is a mistake, and it is named.
func bundleIcon(set *settings) ([]byte, string, error) {
	name := set.file.Favicon
	if name == "" {
		return nil, "", nil
	}
	inside, err := blog.FaviconName(name)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(name)
	if err != nil {
		data, err = os.ReadFile(set.resolve(name))
		if err != nil {
			return nil, "", fmt.Errorf("the favicon %s is not there: %w", name, err)
		}
	}
	return data, inside, nil
}

// bundleFolder gives the folder the posts are read from, for the bundle
// command.
//
// The posts inside the program are written out to a temporary folder first,
// so that a bundle of a bundle carries one set of content and not two.
func (w *work) bundleFolder(set *settings) (string, func(), error) {
	folder, inside, err := w.postsFolder(set)
	if err != nil {
		return "", func() {}, err
	}
	if !inside {
		return folder, func() {}, nil
	}
	files, err := w.bundledContent()
	if err != nil {
		return "", func() {}, err
	}
	if files == nil {
		return "", func() {}, errors.New(
			"no posts to put inside: name a folder, or set BAUMKUCHEN_POSTS")
	}
	dir, err := os.MkdirTemp("", "baumkuchen-bundle-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	if err := copyOut(files, bundle.Root, dir); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return dir, cleanup, nil
}

// copyOut writes the files of a file set into a folder.
func copyOut(files fs.FS, root, dir string) error {
	return fs.WalkDir(files, root, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(name, root+"/")
		if rel == name {
			return nil // the root of the set itself
		}
		target := filepath.Join(dir, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// hasMarkdown says whether a folder holds any markdown. A bundle of nothing
// is not a bundle.
func hasMarkdown(folder string) error {
	files, err := content.NewDir(folder).List()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("%s holds no markdown: there is nothing to put inside", folder)
	}
	return nil
}

// check says what is wrong with the posts.
func (w *work) check() error {
	site, err := w.site(true)
	if err != nil {
		return err
	}
	report := site.Check()
	if w.cmd.was("json") {
		enc := json.NewEncoder(w.out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
	} else {
		writeReport(w.out, report)
	}
	if report.Errors > 0 {
		return errQuiet
	}
	if w.cmd.was("strict") && report.Warnings > 0 {
		return errQuiet
	}
	return nil
}

// writeReport gives the report as text: the counts, then every finding.
func writeReport(w io.Writer, report blog.Report) {
	fmt.Fprintf(w, "%d published post(s), %d draft(s), %d without a depth\n",
		report.Published, report.Drafts, report.Incomplete)
	for _, f := range report.Findings {
		fmt.Fprintf(w, "%s: %s\n", f.Level, f.Message)
	}
	fmt.Fprintf(w, "%d error(s), %d warning(s)\n", report.Errors, report.Warnings)
}

// init writes the settings file for a new site.
func (w *work) init() error {
	dir := w.cmd.folder
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, settingsName)
	if _, err := os.Stat(path); err == nil && !w.cmd.was("force") {
		return fmt.Errorf("%s is already there: give --force to write over it", path)
	}
	if err := os.WriteFile(path, []byte(starterSettings), 0o644); err != nil {
		return err
	}
	posts := filepath.Join(dir, "posts")
	if err := os.MkdirAll(posts, 0o755); err != nil {
		return err
	}
	sample := filepath.Join(posts, "hello.md")
	if _, err := os.Stat(sample); err != nil {
		if err := os.WriteFile(sample, []byte(starterPost), 0o644); err != nil {
			return err
		}
	}
	fmt.Fprintf(w.out, "wrote %s, the %s folder and a first post in it\n", path, posts)
	return nil
}

// version says which build this is.
func (w *work) version() error {
	fmt.Fprintln(w.out, versionLine())
	return nil
}
