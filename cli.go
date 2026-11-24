package main

import (
	"fmt"
	"strconv"
	"strings"
)

// flagDef is one flag a command understands: its long name, its short name
// when it has one, whether it takes a value, and the word that names the
// value in the help.
type flagDef struct {
	name  string
	short string
	arg   bool
	word  string
	desc  string
}

// The flags every command understands.
var globalFlags = []flagDef{
	{name: "config", arg: true, word: "PATH", desc: "the settings file to read"},
	{name: "quiet", short: "q", desc: "say nothing but the result"},
	{name: "help", short: "h", desc: "show this help and stop"},
	{name: "version", short: "V", desc: "say which build this is"},
}

// command is one command of the program.
//
// Everything the program says about a command is here: what it is called,
// what it says about itself, how it is written, the flags it takes, and
// what it does. A new command is one piece added to the list below.
type command struct {
	// name is what the command is called on the command line.
	name string
	// summary is the one line that says what the command does.
	summary string
	// usage is the line that says how the command is written.
	usage string
	// flags are the flags the command takes.
	flags []flagDef
	// run does the work of the command.
	run func(*work) error
}

// commandList gives the commands of the program, in the order the help shows
// them.
//
// It is a function and not a variable because the help of the program is
// built from this list, and this list holds the work of the help. A
// variable would be built out of itself, and the program would not build.
func commandList() []command {
	return []command{
		{
			name:    "serve",
			summary: "serve the posts, live (the command that stands in for the others)",
			usage:   "  baumkuchen serve [folder] [flags]\n",
			flags: []flagDef{
				{name: "addr", arg: true, word: "ADDR", desc: "the address to listen on"},
				{name: "open", desc: "open the site in a browser"},
				{name: "drafts", desc: "show the drafts, wherever it listens"},
				{name: "no-drafts", desc: "hide the drafts, even on the loopback address"},
				{name: "dev", desc: "read the templates and the stylesheet from disk, and show every error"},
				{name: "title", arg: true, word: "TEXT", desc: "the name of the site"},
				{name: "heading", arg: true, word: "TEXT", desc: "the line under the wordmark"},
				{name: "favicon", arg: true, word: "PATH", desc: "the picture on the tab"},
			},
			run: (*work).serve,
		},
		{
			name:    "build",
			summary: "write the static site, then stop",
			usage:   "  baumkuchen build [folder] [flags]\n",
			flags: []flagDef{
				{name: "out", short: "o", arg: true, word: "DIR", desc: "the folder to write the site into"},
				{name: "base", arg: true, word: "PATH", desc: "the folder the site is published in"},
				{name: "clean", desc: "remove the files of an earlier build first"},
				{name: "drafts", desc: "put the drafts in the built site"},
				{name: "title", arg: true, word: "TEXT", desc: "the name of the site"},
				{name: "heading", arg: true, word: "TEXT", desc: "the line under the wordmark"},
				{name: "favicon", arg: true, word: "PATH", desc: "the picture on the tab"},
			},
			run: (*work).build,
		},
		{
			name:    "bundle",
			summary: "write one file with the program and the posts inside it",
			usage:   "  baumkuchen bundle [folder] --out FILE\n",
			flags: []flagDef{
				{name: "out", short: "o", arg: true, word: "FILE", desc: "the file to write"},
			},
			run: (*work).bundle,
		},
		{
			name:    "check",
			summary: "say what is wrong with the posts",
			usage:   "  baumkuchen check [folder] [flags]\n",
			flags: []flagDef{
				{name: "json", desc: "give the report as JSON"},
				{name: "strict", desc: "stop with 1 when there is a warning"},
			},
			run: (*work).check,
		},
		{
			name:    "init",
			summary: "write the settings file for a new site",
			usage:   "  baumkuchen init [folder] [flags]\n",
			flags: []flagDef{
				{name: "force", desc: "write over a settings file that is already there"},
			},
			run: (*work).init,
		},
		{
			name:    "version",
			summary: "say which build this is",
			usage:   "  baumkuchen version\n",
			run:     (*work).version,
		},
		{
			name:    "help",
			summary: "show this help and stop",
			usage:   "  baumkuchen help [command]\n",
			run:     (*work).help,
		},
	}
}

// The command that runs when none is named.
const defaultCommand = "serve"

// The flags that were taken away, and what to write instead.
var removedFlags = map[string]string{
	"export": "baumkuchen build -o DIR",
	"bundle": "baumkuchen bundle -o FILE",
	"posts":  "baumkuchen serve FOLDER",
}

// usageError is a mistake in the command line. The program says what to do
// and stops with the second code.
type usageError struct{ message string }

func (e usageError) Error() string { return e.message }

// usagef makes a mistake in the command line.
func usagef(format string, args ...any) error {
	return usageError{message: fmt.Sprintf(format, args...)}
}

// commandLine is the command line, read.
type commandLine struct {
	name string
	// named says whether the command was written out, rather than being the
	// one that stands in for it.
	named  bool
	folder string
	// values holds every flag that was given, by its long name. A flag that
	// was not given is not in here, so the map says what was asked for and
	// what was answered.
	values map[string]string
	// subject is the command the help is about, when the help command was
	// given the name of one.
	subject string
}

// bool says whether a flag of the command is on.
//
// A flag that was given as --name=false is off, even though it was given:
// the writer asked for it to be off, and that is what is asked for.
func (c *commandLine) bool(name string) bool {
	value, ok := c.values[name]
	if !ok {
		return false
	}
	on, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return on
}

// text gives the value of a flag of the command, and whether it was given.
func (c *commandLine) text(name string) (string, bool) {
	value, ok := c.values[name]
	return value, ok
}

// isCommand says whether a word is the name of a command.
func isCommand(word string) bool {
	_, ok := commandByName(word)
	return ok
}

// commandByName gives the command of that name.
func commandByName(name string) (command, bool) {
	for _, c := range commandList() {
		if c.name == name {
			return c, true
		}
	}
	return command{}, false
}

// knownFlags gives every flag the program knows, by the name it is written
// on the command line: the long name, and the short one when it has one.
func knownFlags() map[string]flagDef {
	table := map[string]flagDef{}
	add := func(list []flagDef) {
		for _, d := range list {
			table[d.name] = d
			if d.short != "" {
				table[d.short] = d
			}
		}
	}
	add(globalFlags)
	for _, c := range commandList() {
		add(c.flags)
	}
	return table
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
	c, ok := commandByName(command)
	if !ok {
		return false
	}
	for _, d := range c.flags {
		if d.name == def.name {
			return true
		}
	}
	return false
}

// flagList gives the flags a command understands: the ones every command
// understands, and the ones of that command.
func flagList(command string) []flagDef {
	out := make([]flagDef, 0, len(globalFlags))
	out = append(out, globalFlags...)
	if c, ok := commandByName(command); ok {
		out = append(out, c.flags...)
	}
	return out
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
//
// The short flags may be written together, as in -qh. Only the last one of
// them can take a value, so the value of a group is written after it, as in
// -qo out.
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
				// A group of short flags: -qh is -q -h. Only the last one
				// of them can take a value, so -qo out is the way to write
				// it.
				if strings.HasPrefix(arg, "--") || len(name) < 2 {
					// the flag is not known: it is named once the
					// command is known, so that the hint can be
					// about the flags of that command
					flags = append(flags, given{name: name})
					continue
				}
				for j := 0; j < len(name); j++ {
					letter := string(name[j])
					def, ok := table[letter]
					if !ok {
						return nil, usagef("unknown flag %s", "-"+letter)
					}
					if !def.arg {
						flags = append(flags, given{name: def.name, value: "true"})
						continue
					}
					if j < len(name)-1 {
						return nil, usagef("-%s needs a value: only the last flag "+
							"of a group can take one", letter)
					}
					if !hasValue {
						if i+1 >= len(argv) {
							return nil, usagef("%s needs a value", "--"+def.name)
						}
						i++
						value = argv[i]
					}
					flags = append(flags, given{name: def.name, value: value})
				}
				continue
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
		values: map[string]string{},
	}
	// the help command names the command its help is about, and that name
	// is not a folder
	if name == "help" {
		cmd.subject, cmd.folder = cmd.folder, ""
	}
	for _, f := range flags {
		def, known := table[f.name]
		if !known {
			return nil, usagef("unknown flag %s%s", "--"+f.name, flagHint(f.name, name))
		}
		if !isGlobal(def) && !belongsTo(def, name) {
			return nil, usagef("unknown flag %s for %s%s",
				"--"+def.name, name, flagHint(def.name, name))
		}
		if !def.arg {
			// a switch takes true or false, and a value that is neither
			// of them is a mistake on the command line
			if _, err := strconv.ParseBool(f.value); err != nil {
				return nil, usagef("%s: %s is not true or false", "--"+def.name, f.value)
			}
		}
		cmd.values[def.name] = f.value
	}
	// the two flags that say the opposite of each other cannot both be
	// given: the writer asked for two things at once.
	_, drafts := cmd.text("drafts")
	_, noDrafts := cmd.text("no-drafts")
	if drafts && noDrafts {
		return nil, usagef("--drafts and --no-drafts cannot be used together")
	}
	return cmd, nil
}

// readCommand says which command to run, and which folder was named.
//
// The first word is the command, unless it is the name of a folder. A word
// that is close to the name of a command, and that no folder carries, is
// most likely a command that was mistyped, and the command it was meant to
// be is named.
func readCommand(words []string) (name, folder string, named bool, err error) {
	name = defaultCommand
	rest := words
	if len(words) > 0 && isCommand(words[0]) {
		name, named = words[0], true
		rest = words[1:]
	} else if len(words) > 0 {
		if near, ok := nearCommand(words[0]); ok && !folderExists(words[0]) {
			return "", "", false, usagef("unknown command %s: did you mean `%s`?",
				words[0], near)
		}
	}
	// the help command takes the name of a command, not a folder
	if name == "help" {
		if len(rest) > 1 {
			return "", "", false, usagef("expected at most one command, got %d: %s",
				len(rest), strings.Join(rest, " "))
		}
		if len(rest) == 1 {
			if !isCommand(rest[0]) {
				return "", "", false, usagef("unknown command %s", rest[0])
			}
			return name, rest[0], named, nil
		}
		return name, "", named, nil
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

// usageText gives the help: the whole program, or one command.
func usageText(command string, named bool) string {
	if named {
		return commandUsage(command)
	}
	return programUsage()
}

// programUsage gives the help for the whole program.
func programUsage() string {
	var b strings.Builder
	b.WriteString("baumkuchen — a blog that carries the same idea at up to five lengths\n\n")
	b.WriteString("Usage:\n")
	for _, c := range commandList() {
		b.WriteString(programLine(c))
	}
	b.WriteString("\nCommands:\n")
	for _, c := range commandList() {
		fmt.Fprintf(&b, "  %-8s %s\n", c.name, c.summary)
	}
	b.WriteString("\nGlobal flags:\n")
	b.WriteString(flagLines(globalFlags))
	b.WriteString("\nThe folder is the one with the markdown in it. It may come before or after\n")
	b.WriteString("the flags. A folder whose name is the name of a command is written with a\n")
	b.WriteString("path in front of it, as in ./build.\n")
	b.WriteString("\nRun \"baumkuchen <command> --help\" for the flags of one command.\n")
	return b.String()
}

// programLine gives the line of the help of the whole program for one
// command.
//
// The command that stands in for the others is written in brackets, because
// it may be left out.
func programLine(c command) string {
	if c.name != defaultCommand {
		return c.usage
	}
	return strings.Replace(c.usage, "baumkuchen "+c.name, "baumkuchen ["+c.name+"]", 1)
}

// commandUsage gives the help for one command.
func commandUsage(name string) string {
	c, ok := commandByName(name)
	if !ok {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "baumkuchen %s — %s\n\n", c.name, c.summary)
	b.WriteString("Usage:\n")
	b.WriteString(c.usage)
	b.WriteString("\nFlags:\n")
	b.WriteString(flagLines(c.flags))
	b.WriteString("\nGlobal flags:\n")
	b.WriteString(flagLines(globalFlags))
	return b.String()
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
			name += " " + d.word
		}
		fmt.Fprintf(&b, "  %-22s %s\n", name, d.desc)
	}
	return b.String()
}
