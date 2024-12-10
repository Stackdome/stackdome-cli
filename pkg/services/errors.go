package services

import "fmt"

type ServiceError struct {
	Message       string
	OriginalError error
	Code          int
}

func (e *ServiceError) Error() string {
	if code := e.Code; code != 0 {
		return fmt.Sprintf("%s: %s (http error code: %d)", e.Message, e.OriginalError.Error(), code)
	}
	return fmt.Sprintf("%s: %s", e.Message, e.OriginalError.Error())
}

func NewServiceError(err error) *ServiceError {
	return &ServiceError{
		Message:       "service error",
		OriginalError: err,
	}
}

func NewServiceErrorWithCode(err error, code int) *ServiceError {
	return &ServiceError{
		Message:       "service error",
		OriginalError: err,
		Code:          code,
	}
}

// With message and code
func NewServiceErrorWithMessageAndCode(err error, message string, code int) *ServiceError {
	return &ServiceError{
		Message:       message,
		OriginalError: err,
		Code:          code,
	}
}

func NewServiceErrorWithMessage(err error, message string) *ServiceError {
	return &ServiceError{
		Message:       message,
		OriginalError: err,
	}
}
