// Saying the word the writer meant: when a word on the command line is
// close to one the program knows, it names it.
//
// The counting is done here, in the program, because there is no library to
// ask and the program does not take a new dependency for this.
package main

import (
	"fmt"
	"os"
	"strings"
)

// The most edits two words may be apart and still be worth naming.
const nearDistance = 2

// editDistance gives how many letters must be changed to make one word into
// another: the fewest number of changes, where a change is a letter put in,
// a letter taken out, or a letter turned into another one.
//
// The counting is the plain one, and it is here because the program asks for
// the word the writer meant.
func editDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	previous := make([]int, len(br)+1)
	current := make([]int, len(br)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		current[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			current[j] = min(current[j-1]+1, previous[j]+1, previous[j-1]+cost)
		}
		previous, current = current, previous
	}
	return previous[len(br)]
}

// nearCommand gives the command a mistyped word is closest to, when the two
// are close enough to be worth naming.
//
// A word with a path in it is left alone: ./build is a folder that was named
// on purpose, and not a command that was mistyped.
func nearCommand(word string) (string, bool) {
	if strings.ContainsAny(word, "/\\") {
		return "", false
	}
	best, distance := "", nearDistance+1
	for _, c := range commandList() {
		if d := editDistance(word, c.name); d < distance {
			best, distance = c.name, d
		}
	}
	return best, best != ""
}

// nearFlag gives the flag of a command a mistyped flag is closest to, when
// the two are close enough to be worth naming.
//
// The flags of the command are the ones it understands, and the ones every
// command understands, because a writer may use either of them.
func nearFlag(word, command string) (string, bool) {
	best, distance := "", nearDistance+1
	for _, def := range flagList(command) {
		if d := editDistance(word, def.name); d < distance {
			best, distance = def.name, d
		}
	}
	return best, best != ""
}

// flagHint says the flag the writer may have meant, when there is one that
// is close enough to be worth naming.
func flagHint(word, command string) string {
	near, ok := nearFlag(word, command)
	if !ok {
		return ""
	}
	return fmt.Sprintf(": did you mean `%s`?", "--"+near)
}

// folderExists says whether a folder of that name is there.
//
// A word that names a folder is a folder, and not a word that was mistyped:
// the writer meant the folder, and the program uses it.
func folderExists(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.IsDir()
}
