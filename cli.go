package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// flagDef is one flag a command understands: its long name, its short name
// when it has one, and whether it takes a value.
type flagDef struct {
	name  string
	short string
	arg   bool
	desc  string
}

// The flags every command understands.
var globalFlags = []flagDef{
	{name: "config", arg: true, desc: "the settings file to read"},
	{name: "quiet", short: "q", desc: "say nothing but the result"},
	{name: "verbose", short: "v", desc: "say more about what is going on"},
	{name: "help", short: "h", desc: "show this help and stop"},
}

// The flags the commands add.
var commandFlags = map[string][]flagDef{
	"serve": {
		{name: "addr", arg: true, desc: "the address to listen on"},
		{name: "open", desc: "open the site in a browser"},
		{name: "drafts", desc: "show the drafts, wherever it listens"},
		{name: "no-drafts", desc: "hide the drafts, even on the loopback address"},
		{name: "dev", desc: "read the templates and the stylesheet from disk, and show every error"},
		{name: "title", arg: true, desc: "the name of the site"},
		{name: "heading", arg: true, desc: "the line under the wordmark"},
		{name: "favicon", arg: true, desc: "the picture on the tab"},
	},
	"build": {
		{name: "out", short: "o", arg: true, desc: "the folder to write the site into"},
		{name: "base", arg: true, desc: "the folder the site is published in"},
		{name: "clean", desc: "remove the files of an earlier build first"},
		{name: "drafts", desc: "put the drafts in the built site"},
		{name: "title", arg: true, desc: "the name of the site"},
		{name: "heading", arg: true, desc: "the line under the wordmark"},
		{name: "favicon", arg: true, desc: "the picture on the tab"},
	},
	"bundle": {
		{name: "out", short: "o", arg: true, desc: "the file to write"},
	},
	"check": {
		{name: "json", desc: "give the report as JSON"},
		{name: "strict", desc: "stop with 1 when there is a warning"},
	},
	"init": {
		{name: "force", desc: "write over a settings file that is already there"},
	},
	"version": {},
}

// The commands, in the order the help shows them.
var commandNames = []string{"serve", "build", "bundle", "check", "init", "version"}

// The command that runs when none is named.
const defaultCommand = "serve"

// The flags that were taken away, and what to write instead.
var removedFlags = map[string]string{
	"export":  "baumkuchen build -o DIR",
	"bundle":  "baumkuchen bundle -o FILE",
	"posts":   "baumkuchen serve FOLDER",
	"version": "baumkuchen version",
}

// usageError is a mistake in the command line. The program says what to do
// and stops with the second code.
type usageError struct{ message string }

func (e usageError) Error() string { return e.message }

// usagef makes a mistake in the command line.
func usagef(format string, args ...any) error {
	return usageError{message: fmt.Sprintf(format, args...)}
}

// globals are the flags every command understands.
type globals struct {
	config  string
	quiet   bool
	verbose bool
	help    bool
}

// commandLine is the command line, read.
type commandLine struct {
	name string
	// named says whether the command was written out, rather than being the
	// one that stands in for it.
	named   bool
	folder  string
	globals globals
	flags   *flag.FlagSet
	given   map[string]bool
}

// value gives the value of a flag of the command, and whether it was given.
func (c *commandLine) value(name string) (string, bool) {
	if !c.given[name] {
		return "", false
	}
	f := c.flags.Lookup(name)
	if f == nil {
		return "", false
	}
	return f.Value.String(), true
}

// was says whether a flag of the command was given.
func (c *commandLine) was(name string) bool { return c.given[name] }

// isCommand says whether a word is the name of a command.
func isCommand(word string) bool {
	_, ok := commandFlags[word]
	return ok
}

// knownFlags gives every flag the program knows, by the name it is written
// on the command line: the long name, and the short one when it has one.
func knownFlags() map[string]flagDef {
	table := map[string]flagDef{}
	for _, list := range append([][]flagDef{globalFlags}, commandLists()...) {
		for _, d := range list {
			table[d.name] = d
			if d.short != "" {
				table[d.short] = d
			}
		}
	}
	return table
}

// commandLists gives the flags of every command.
func commandLists() [][]flagDef {
	out := make([][]flagDef, 0, len(commandFlags))
	for _, name := range commandNames {
		out = append(out, commandFlags[name])
	}
	return out
}

// isGlobal says whether a flag is one every command understands.
func isGlobal(def flagDef) bool {
	for _, g := range globalFlags {
		if g.name == def.name {
			return true
		}
	}
	return false
}

// belongsTo says whether a flag is one of the flags of a command.
func belongsTo(def flagDef, command string) bool {
	for _, d := range commandFlags[command] {
		if d.name == def.name {
			return true
		}
	}
	return false
}

// splitFlag takes a flag off the command line: its name, and the value that
// came with it, if it came with one.
func splitFlag(arg string) (name, value string, hasValue bool) {
	trimmed := strings.TrimLeft(arg, "-")
	if i := strings.Index(trimmed, "="); i >= 0 {
		return trimmed[:i], trimmed[i+1:], true
	}
	return trimmed, "", false
}

// readArgs reads the command line.
//
// The command comes first, but the flags may come before it or after it, and
// the folder may come before the flags or after them. A folder whose name is
// the name of a command has to be written with a path in front of it, as in
// ./build, so that there is never a doubt about what it is.
func readArgs(argv []string) (*commandLine, error) {
	table := knownFlags()
	type given struct {
		name  string
		value string
	}
	var flags []given
	var words []string
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "--" {
			words = append(words, argv[i+1:]...)
			break
		}
		if len(arg) > 1 && strings.HasPrefix(arg, "-") {
			name, value, hasValue := splitFlag(arg)
			if hint, gone := removedFlags[name]; gone {
				return nil, usagef("-%s is gone: use %s", name, hint)
			}
			def, known := table[name]
			if !known {
				return nil, usagef("unknown flag %s", "--"+name)
			}
			if def.arg && !hasValue {
				if i+1 >= len(argv) {
					return nil, usagef("%s needs a value", "--"+def.name)
				}
				i++
				value = argv[i]
			}
			if !def.arg && !hasValue {
				value = "true"
			}
			flags = append(flags, given{name: name, value: value})
			continue
		}
		words = append(words, arg)
	}

	name, folder, named, err := readCommand(words)
	if err != nil {
		return nil, err
	}
	cmd := &commandLine{
		name:   name,
		named:  named,
		folder: folder,
		flags:  newFlagSet(name),
		given:  map[string]bool{},
	}
	for _, f := range flags {
		def := table[f.name]
		if isGlobal(def) {
			cmd.setGlobal(def.name, f.value)
			continue
		}
		if !belongsTo(def, name) {
			return nil, usagef("unknown flag %s for %s", "--"+def.name, name)
		}
		if err := cmd.flags.Set(def.name, f.value); err != nil {
			return nil, usagef("%s: %v", "--"+def.name, err)
		}
		cmd.given[def.name] = true
	}
	return cmd, nil
}

// readCommand says which command to run, and which folder was named.
func readCommand(words []string) (name, folder string, named bool, err error) {
	name = defaultCommand
	rest := words
	if len(words) > 0 && isCommand(words[0]) {
		name, named = words[0], true
		rest = words[1:]
	}
	if len(rest) > 1 {
		return "", "", false, usagef("expected at most one folder, got %d: %s",
			len(rest), strings.Join(rest, " "))
	}
	if len(rest) == 1 {
		folder = rest[0]
		if isCommand(folder) {
			return "", "", false, usagef(
				"%s is the name of a command: write the folder as ./%s", folder, folder)
		}
	}
	return name, folder, named, nil
}

// newFlagSet makes the flags of a command.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	for _, d := range commandFlags[name] {
		if d.arg {
			fs.String(d.name, "", d.desc)
		} else {
			fs.Bool(d.name, false, d.desc)
		}
	}
	return fs
}

// setGlobal puts a global flag where it belongs.
func (c *commandLine) setGlobal(name, value string) {
	switch name {
	case "config":
		c.globals.config = value
	case "quiet":
		c.globals.quiet = value == "true"
	case "verbose":
		c.globals.verbose = value == "true"
	case "help":
		c.globals.help = value == "true"
	}
}

// usageText gives the help: the whole program, or one command.
func usageText(command string, named bool) string {
	if named {
		return commandUsage(command)
	}
	var b strings.Builder
	b.WriteString("baumkuchen — a blog that carries the same idea at up to five lengths\n\n")
	b.WriteString("Usage:\n")
	b.WriteString("  baumkuchen [serve] [folder] [flags]\n")
	b.WriteString("  baumkuchen build [folder] [flags]\n")
	b.WriteString("  baumkuchen bundle [folder] --out FILE\n")
	b.WriteString("  baumkuchen check [folder] [flags]\n")
	b.WriteString("  baumkuchen init [folder] [flags]\n")
	b.WriteString("  baumkuchen version\n\n")
	b.WriteString("Commands:\n")
	for _, name := range commandNames {
		fmt.Fprintf(&b, "  %-8s %s\n", name, commandDescription(name))
	}
	b.WriteString("\nGlobal flags:\n")
	b.WriteString(flagLines(globalFlags))
	b.WriteString("\nThe folder is the one with the markdown in it. It may come before or after\n")
	b.WriteString("the flags. A folder whose name is the name of a command is written with a\n")
	b.WriteString("path in front of it, as in ./build.\n")
	b.WriteString("\nRun \"baumkuchen <command> --help\" for the flags of one command.\n")
	return b.String()
}

// commandUsage gives the help for one command.
func commandUsage(command string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "baumkuchen %s — %s\n\n", command, commandDescription(command))
	b.WriteString("Usage:\n")
	switch command {
	case "serve":
		b.WriteString("  baumkuchen serve [folder] [flags]\n")
	case "build":
		b.WriteString("  baumkuchen build [folder] [flags]\n")
	case "bundle":
		b.WriteString("  baumkuchen bundle [folder] --out FILE\n")
	case "check":
		b.WriteString("  baumkuchen check [folder] [flags]\n")
	case "init":
		b.WriteString("  baumkuchen init [folder] [flags]\n")
	case "version":
		b.WriteString("  baumkuchen version\n")
	}
	b.WriteString("\nFlags:\n")
	b.WriteString(flagLines(commandFlags[command]))
	b.WriteString("\nGlobal flags:\n")
	b.WriteString(flagLines(globalFlags))
	return b.String()
}

// commandDescription gives the one line that says what a command does.
func commandDescription(command string) string {
	switch command {
	case "serve":
		return "serve the posts, live (the command that stands in for the others)"
	case "build":
		return "write the static site, then stop"
	case "bundle":
		return "write one file with the program and the posts inside it"
	case "check":
		return "say what is wrong with the posts"
	case "init":
		return "write the settings file for a new site"
	case "version":
		return "say which build this is"
	}
	return ""
}

// flagLines gives the flags as they are written in the help.
func flagLines(defs []flagDef) string {
	var b strings.Builder
	for _, d := range defs {
		name := "--" + d.name
		if d.short != "" {
			name = "-" + d.short + ", " + name
		}
		if d.arg {
			name += " FILE"
		}
		fmt.Fprintf(&b, "  %-22s %s\n", name, d.desc)
	}
	return b.String()
}
