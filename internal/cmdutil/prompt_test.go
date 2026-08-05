package cmdutil

import (
	"errors"
	"io"
	"strings"
	"testing"

	clierrors "github.com/stackdome/cli/internal/errors"
)

func TestConfirmNonTTYWithoutYesErrors(t *testing.T) {
	_, err := ConfirmFrom(strings.NewReader(""), false /*isTTY*/, false /*assumeYes*/, io.Discard, "delete?")
	if err == nil {
		t.Fatal("expected an error when stdin is not a TTY and --yes is absent")
	}
	if code := clierrors.ExitCodeFrom(err); code != clierrors.ExitValidation {
		t.Errorf("exit code = %d, want %d", code, clierrors.ExitValidation)
	}
}

func TestConfirmNoReturnsCanceled(t *testing.T) {
	ok, err := ConfirmFrom(strings.NewReader("n\n"), true, false, io.Discard, "delete?")
	if ok {
		t.Error("expected ok=false")
	}
	if !errors.Is(err, clierrors.ErrUserCanceled) {
		t.Fatalf("err = %v, want ErrUserCanceled", err)
	}
	if code := clierrors.ExitCodeFrom(err); code != clierrors.ExitUserCanceled {
		t.Errorf("exit code = %d, want %d", code, clierrors.ExitUserCanceled)
	}
}

func TestConfirmYes(t *testing.T) {
	ok, err := ConfirmFrom(strings.NewReader("y\n"), true, false, io.Discard, "delete?")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v, want true/nil", ok, err)
	}
}

func TestConfirmAssumeYesSkipsPrompt(t *testing.T) {
	ok, err := ConfirmFrom(strings.NewReader(""), false, true, io.Discard, "delete?")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v, want true/nil", ok, err)
	}
}
