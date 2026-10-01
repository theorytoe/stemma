package cli

import (
	"io"
	"os"
)

// isTerminal reports whether w is a terminal.
//
// A terminal is a character device and a pipe or a redirected file is not,
// which is the whole distinction a command needs: what reads well on a
// terminal is noise in a file, and the tool has to be able to tell the two
// apart before it decorates its output.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// colorEnabled reports whether a command may write ANSI escapes to w.
//
// The test lives in one place so every command that colors agrees about when
// to. Color is for a person watching a terminal, so it is off whenever w is not
// one. NO_COLOR, set to anything but the empty string, turns it off explicitly,
// and a dumb terminal gets the same treatment because it would print the
// escapes as text rather than interpret them.
func colorEnabled(w io.Writer) bool {
	if !isTerminal(w) {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return os.Getenv("TERM") != "dumb"
}

// terminalWidth returns how many columns wide the terminal behind w is, or 0
// when w is not a terminal or the size cannot be read.
//
// Zero means "unknown", and a caller that gets it prints without clipping
// rather than guessing a width it cannot know.
func terminalWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return 0
	}
	return widthOf(f)
}
