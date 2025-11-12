package errors

import "encoding/json"

// HTTPError represents an error response for HTTP APIs
type HTTPError struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"details,omitempty"`
	StackTrace []string       `json:"stack_trace,omitempty"`
}

// ToHTTPError converts an error to HTTPError
func ToHTTPError(err error, includeStackTrace bool) HTTPError {
	if err == nil {
		return HTTPError{
			Code:    CodeInternal.String(),
			Message: "Unknown error",
		}
	}

	var customErr *Error
	if As(err, &customErr) {
		httpErr := HTTPError{
			Code:    customErr.Code.String(),
			Message: customErr.Message,
			Details: customErr.Details,
		}

		if includeStackTrace && len(customErr.StackTrace) > 0 {
			traces := make([]string, 0, len(customErr.StackTrace))
			for _, frame := range customErr.StackTrace {
				traces = append(traces, frame.String())
			}
			httpErr.StackTrace = traces
		}

		return httpErr
	}

	return HTTPError{
		Code:    CodeInternal.String(),
		Message: err.Error(),
	}
}

// ToJSON converts HTTPError to JSON string
func (e HTTPError) ToJSON() (string, error) {
	bytes, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// HTTPStatusCode returns the HTTP status code for an error
func HTTPStatusCode(err error) int {
	if err == nil {
		return 200
	}

	var customErr *Error
	if As(err, &customErr) {
		return customErr.Code.HTTPStatusCode()
	}

	return 500
}

// HTTPResponse represents a complete HTTP error response
type HTTPResponse struct {
	StatusCode int       `json:"-"`
	Error      HTTPError `json:"error"`
}

// ToHTTPResponse converts an error to HTTPResponse
func ToHTTPResponse(err error, includeStackTrace bool) HTTPResponse {
	httpErr := ToHTTPError(err, includeStackTrace)
	statusCode := HTTPStatusCode(err)

	return HTTPResponse{
		StatusCode: statusCode,
		Error:      httpErr,
	}
}

// WriteJSON writes the HTTP response as JSON
func (r HTTPResponse) WriteJSON() (string, error) {
	bytes, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
