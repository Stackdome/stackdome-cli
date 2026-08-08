package errors

import "testing"

func TestUserMessageDoesNotExposeInternalDetail(t *testing.T) {
	err := &CLIError{
		Message:  "Resource not found",
		Detail:   "internal namespace and workload diagnostics",
		Code:     "NOT_FOUND",
		ExitCode: ExitNotFound,
	}

	if got, want := UserMessage(err), "Resource not found"; got != want {
		t.Fatalf("UserMessage() = %q, want %q", got, want)
	}
}
