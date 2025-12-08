package errors

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToGRPCError converts an error to a gRPC error
func ToGRPCError(err error) error {
	if err == nil {
		return nil
	}

	var customErr *Error
	if As(err, &customErr) {
		code := codes.Code(customErr.Code.GRPCCode())
		st := status.New(code, customErr.Message)

		// Ideally we would add details here, but we need to convert map[string]any to proto.Message
		// which is non-trivial without knowing the specific proto messages.
		// For now we just return the status with code and message.
		return st.Err()
	}

	return status.Error(codes.Internal, err.Error())
}
