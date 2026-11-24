package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The help is a contract: what it says is what the program does. The golden
// file holds it, so a change to the help is a change on purpose and not by
// accident.
func TestTheHelpMatchesTheGolden(t *testing.T) {
	// arrange
	golden, err := os.ReadFile(filepath.Join("testdata", "help.golden"))
	if err != nil {
		t.Fatalf("the golden file cannot be read: %v", err)
	}

	// act
	var got bytes.Buffer
	for _, command := range []string{"", "serve", "build", "bundle", "check", "init", "version", "help"} {
		heading := "===== baumkuchen (no command) --help =====\n"
		if command != "" {
			heading = fmt.Sprintf("===== baumkuchen %s --help =====\n", command)
		}
		got.WriteString(heading)
		argv := []string{"--help"}
		if command != "" {
			argv = []string{command, "--help"}
		}
		var stdout, stderr bytes.Buffer
		if code := run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("the help of %s gave the code %d (the error stream said: %s)",
				command, code, stderr.String())
		}
		got.WriteString(strings.TrimRight(stdout.String(), "\n"))
		got.WriteString("\n")
	}

	// assert
	if got.String() != string(golden) {
		t.Errorf("the help is not the one in the golden file:\n%s", got.String())
	}
}
