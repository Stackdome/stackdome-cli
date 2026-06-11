package errors

import (
	"errors"
	"fmt"
)

const (
	ExitGeneral      = 1
	ExitAuth         = 2
	ExitNotFound     = 3
	ExitValidation   = 4
	ExitConflict     = 5
	ExitUserCanceled = 130
)

type CLIError struct {
	Message  string
	Detail   string
	Code     string
	ExitCode int
	Cause    error
}

func (e *CLIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s", e.Message, e.Detail)
	}
	return e.Message
}

func (e *CLIError) Unwrap() error {
	return e.Cause
}

func New(message string) *CLIError {
	return &CLIError{
		Message:  message,
		ExitCode: ExitGeneral,
	}
}

func Newf(format string, args ...any) *CLIError {
	return &CLIError{
		Message:  fmt.Sprintf(format, args...),
		ExitCode: ExitGeneral,
	}
}

func Wrap(err error, message string) *CLIError {
	return &CLIError{
		Message:  message,
		ExitCode: ExitGeneral,
		Cause:    err,
	}
}

func Wrapf(err error, format string, args ...any) *CLIError {
	return &CLIError{
		Message:  fmt.Sprintf(format, args...),
		ExitCode: ExitGeneral,
		Cause:    err,
	}
}

func (e *CLIError) WithCode(code string) *CLIError {
	e.Code = code
	return e
}

func (e *CLIError) WithDetail(detail string) *CLIError {
	e.Detail = detail
	return e
}

func (e *CLIError) WithExitCode(code int) *CLIError {
	e.ExitCode = code
	return e
}

func AuthError(message string) *CLIError {
	return &CLIError{
		Message:  message,
		Code:     "AUTH_ERROR",
		ExitCode: ExitAuth,
	}
}

func NotFoundError(resource, name string) *CLIError {
	return &CLIError{
		Message:  fmt.Sprintf("%s %q not found", resource, name),
		Code:     "NOT_FOUND",
		ExitCode: ExitNotFound,
	}
}

func ValidationError(message string) *CLIError {
	return &CLIError{
		Message:  message,
		Code:     "VALIDATION_ERROR",
		ExitCode: ExitValidation,
	}
}

func FromHTTP(statusCode int, body string) *CLIError {
	e := &CLIError{
		Detail: body,
	}
	switch {
	case statusCode == 401:
		e.Message = "Authentication failed. Run `stackdome login` to re-authenticate."
		e.Code = "AUTH_EXPIRED"
		e.ExitCode = ExitAuth
	case statusCode == 403:
		e.Message = "Permission denied. Your session may have expired — run `stackdome login` to re-authenticate."
		e.Code = "FORBIDDEN"
		e.ExitCode = ExitAuth
	case statusCode == 404:
		e.Message = "Resource not found"
		e.Code = "NOT_FOUND"
		e.ExitCode = ExitNotFound
	case statusCode == 409:
		e.Message = "Conflict"
		e.Code = "CONFLICT"
		e.ExitCode = ExitConflict
	case statusCode == 400:
		e.Message = body
		if e.Message == "" {
			e.Message = "Invalid request"
		}
		e.Code = "VALIDATION_ERROR"
		e.ExitCode = ExitValidation
	case statusCode >= 500:
		e.Message = "Server error — try again later"
		e.Code = "SERVER_ERROR"
		e.ExitCode = ExitGeneral
	default:
		e.Message = fmt.Sprintf("Request failed (HTTP %d)", statusCode)
		e.Code = "REQUEST_FAILED"
		e.ExitCode = ExitGeneral
	}
	return e
}

func ExitCodeFrom(err error) int {
	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		return cliErr.ExitCode
	}
	return ExitGeneral
}

func UserMessage(err error) string {
	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		return cliErr.Message
	}
	return err.Error()
}
