package errors

import (
	"errors"
	"fmt"
	"runtime"
)

// Error represents a custom error with additional context
type Error struct {
	Code       Code
	Message    string
	Cause      error
	StackTrace []StackFrame
	Details    map[string]any
}

// StackFrame represents a single frame in the stack trace
type StackFrame struct {
	File     string
	Line     int
	Function string
}

// String returns a string representation of the stack frame
func (sf StackFrame) String() string {
	return sf.Function + " at " + sf.File + ":" + string(rune(sf.Line))
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func (e *Error) WithDetail(key string, value any) *Error {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	e.Details[key] = value
	return e
}

func New(code Code, message string) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		StackTrace: captureStackTrace(),
		Details:    make(map[string]any),
	}
}

func Newf(code Code, format string, args ...any) *Error {
	return &Error{
		Code:       code,
		Message:    fmt.Sprintf(format, args...),
		StackTrace: captureStackTrace(),
		Details:    make(map[string]any),
	}
}

func Wrap(err error, code Code, message string) *Error {
	if err == nil {
		return nil
	}

	var originalErr *Error
	if errors.As(err, &originalErr) {
		details := make(map[string]any)
		for k, v := range originalErr.Details {
			details[k] = v
		}

		return &Error{
			Code:       code,
			Message:    message,
			Cause:      err,
			StackTrace: originalErr.StackTrace,
			Details:    details,
		}
	}

	return &Error{
		Code:       code,
		Message:    message,
		Cause:      err,
		StackTrace: captureStackTrace(),
		Details:    make(map[string]any),
	}
}

func Wrapf(err error, code Code, format string, args ...any) *Error {
	if err == nil {
		return nil
	}

	return Wrap(err, code, fmt.Sprintf(format, args...))
}

func Cause(err error) error {
	for err != nil {
		cause := errors.Unwrap(err)
		if cause == nil {
			return err
		}
		err = cause
	}
	return err
}

func Is(err, target error) bool {
	return errors.Is(err, target)
}

func As(err error, target any) bool {
	return errors.As(err, target)
}

func HasCode(err error, code Code) bool {
	var customErr *Error
	if errors.As(err, &customErr) {
		return customErr.Code == code
	}
	return false
}

func GetCode(err error) Code {
	var customErr *Error
	if errors.As(err, &customErr) {
		return customErr.Code
	}
	return CodeInternal
}

func IsCode(err error, code Code) bool {
	return HasCode(err, code)
}

func GetDetails(err error) map[string]any {
	var customErr *Error
	if errors.As(err, &customErr) {
		return customErr.Details
	}
	return nil
}

func captureStackTrace() []StackFrame {
	const maxDepth = 32
	var pcs [maxDepth]uintptr
	n := runtime.Callers(3, pcs[:])

	frames := make([]StackFrame, 0, n)
	for i := 0; i < n; i++ {
		pc := pcs[i]
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}

		file, line := fn.FileLine(pc)
		frames = append(frames, StackFrame{
			File:     file,
			Line:     line,
			Function: fn.Name(),
		})
	}

	return frames
}
