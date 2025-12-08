package errors

import (
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNew(t *testing.T) {
	err := New(CodeInternal, "internal error")
	if err.Code != CodeInternal {
		t.Errorf("expected code %s, got %s", CodeInternal, err.Code)
	}
	if err.Message != "internal error" {
		t.Errorf("expected message 'internal error', got '%s'", err.Message)
	}
	if err.Cause != nil {
		t.Error("expected cause to be nil")
	}
}

func TestNewf(t *testing.T) {
	err := Newf(CodeInternal, "error %d", 1)
	if err.Message != "error 1" {
		t.Errorf("expected message 'error 1', got '%s'", err.Message)
	}
}

func TestWrap(t *testing.T) {
	baseErr := errors.New("base error")
	err := Wrap(baseErr, CodeDatabase, "wrapper")

	if err.Code != CodeDatabase {
		t.Errorf("expected code %s, got %s", CodeDatabase, err.Code)
	}
	if err.Message != "wrapper" {
		t.Errorf("expected message 'wrapper', got '%s'", err.Message)
	}
	if err.Cause != baseErr {
		t.Error("expected cause to be baseErr")
	}

	// Test Unwrap
	if errors.Unwrap(err) != baseErr {
		t.Error("Unwrap should return baseErr")
	}
}

func TestWrapNil(t *testing.T) {
	err := Wrap(nil, CodeInternal, "msg")
	if err != nil {
		t.Error("Wrap(nil) should return nil")
	}
}

func TestIsCode(t *testing.T) {
	err := New(CodeNotFound, "not found")
	if !IsCode(err, CodeNotFound) {
		t.Error("IsCode should return true")
	}
	if IsCode(err, CodeInternal) {
		t.Error("IsCode should return false for different code")
	}
}

func TestToHTTPError(t *testing.T) {
	err := New(CodeInvalidArgument, "invalid")
	httpErr := ToHTTPError(err, false)

	if httpErr.Code != "INVALID_ARGUMENT" {
		t.Errorf("expected http code INVALID_ARGUMENT, got %s", httpErr.Code)
	}

	status := HTTPStatusCode(err)
	if status != 400 {
		t.Errorf("expected status 400, got %d", status)
	}
}

func TestToGRPCError(t *testing.T) {
	err := New(CodeNotFound, "not found")
	grpcErr := ToGRPCError(err)

	st, ok := status.FromError(grpcErr)
	if !ok {
		t.Fatal("expected grpc status error")
	}

	if st.Code() != codes.NotFound {
		t.Errorf("expected grpc code NotFound, got %s", st.Code())
	}
	if st.Message() != "not found" {
		t.Errorf("expected message 'not found', got '%s'", st.Message())
	}
}

func TestToCMDError(t *testing.T) {
	err := New(CodeTimeout, "timeout")
	cmdErr := ToCMDError(err)

	expected := "[TIMEOUT] timeout"
	if cmdErr != expected {
		t.Errorf("expected '%s', got '%s'", expected, cmdErr)
	}
}

func TestToCMDErrorWithStack(t *testing.T) {
	err := New(CodeInternal, "fail")
	cmdErr := ToCMDErrorWithStack(err)

	if !strings.Contains(cmdErr, "[INTERNAL_ERROR] fail") {
		t.Error("should contain error message")
	}
	if !strings.Contains(cmdErr, "Stack Trace:") {
		t.Error("should contain stack trace header")
	}
}
