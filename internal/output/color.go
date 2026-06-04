package output

import (
	"os"

	"golang.org/x/term"
)

const (
	reset   = "\033[0m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	bold    = "\033[1m"
	dim     = "\033[2m"
)

var noColor bool

func SetNoColor(v bool) {
	noColor = v
}

func isTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func colorize(color, text string) string {
	if noColor || !isTTY() {
		return text
	}
	return color + text + reset
}

func Red(text string) string     { return colorize(red, text) }
func Green(text string) string   { return colorize(green, text) }
func Yellow(text string) string  { return colorize(yellow, text) }
func Blue(text string) string    { return colorize(blue, text) }
func Magenta(text string) string { return colorize(magenta, text) }
func Cyan(text string) string    { return colorize(cyan, text) }
func Bold(text string) string    { return colorize(bold, text) }
func Dim(text string) string     { return colorize(dim, text) }

func StateColor(state string) string {
	switch state {
	case "Ready":
		return Green(state)
	case "Pending":
		return Yellow(state)
	case "Failed", "Error":
		return Red(state)
	case "Deleting":
		return Magenta(state)
	default:
		return state
	}
}
