package cmdutil

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	clierrors "github.com/stackdome/cli/internal/errors"
	"github.com/stackdome/cli/internal/output"
	"golang.org/x/term"
)

// Confirm asks for a y/N confirmation on stderr. assumeYes (--yes) skips the
// prompt entirely; a non-interactive stdin without --yes is an error rather
// than a guess. The formatter is accepted for call-site symmetry only —
// prompts always go to stderr so they never pollute -o json output.
func Confirm(f *output.Formatter, prompt string, assumeYes bool) (bool, error) {
	return ConfirmFrom(os.Stdin, term.IsTerminal(int(os.Stdin.Fd())), assumeYes, os.Stderr, prompt)
}

// ConfirmFrom is the testable core of Confirm.
func ConfirmFrom(r io.Reader, isTTY, assumeYes bool, w io.Writer, prompt string) (bool, error) {
	if assumeYes {
		return true, nil
	}
	if !isTTY {
		return false, clierrors.ValidationError("confirmation required; pass --yes in non-interactive mode")
	}

	fmt.Fprintf(w, "%s [y/N]: ", prompt)
	scanner := bufio.NewScanner(r)
	scanner.Scan()

	switch strings.ToLower(strings.TrimSpace(scanner.Text())) {
	case "y", "yes":
		return true, nil
	}
	return false, clierrors.ErrUserCanceled
}
