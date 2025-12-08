package errors

import (
	"fmt"
	"strings"
)

// ToCMDError formats an error for CLI output
// It returns a colored string if color support is assumed (ANSI codes)
// Format: [CODE] Message
func ToCMDError(err error) string {
	if err == nil {
		return ""
	}

	var customErr *Error
	if As(err, &customErr) {
		return fmt.Sprintf("[%s] %s", customErr.Code, customErr.Message)
	}

	return fmt.Sprintf("[%s] %s", CodeInternal, err.Error())
}

// ToCMDErrorWithStack returns the error message with stack trace
func ToCMDErrorWithStack(err error) string {
	msg := ToCMDError(err)
	if msg == "" {
		return ""
	}

	var customErr *Error
	if As(err, &customErr) && len(customErr.StackTrace) > 0 {
		var sb strings.Builder
		sb.WriteString(msg)
		sb.WriteString("\nStack Trace:\n")
		for _, frame := range customErr.StackTrace {
			sb.WriteString(fmt.Sprintf("  at %s:%d %s\n", frame.File, frame.Line, frame.Function))
		}
		return sb.String()
	}

	return msg
}
