package cdek

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestErrorResponse_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ErrorResponse
		wantText string
	}{
		{
			name:     "no errors",
			err:      &ErrorResponse{Errors: []ErrorDetail{}},
			wantText: "cdek api error: unknown error",
		},
		{
			name: "single error with code",
			err: &ErrorResponse{
				Errors: []ErrorDetail{
					{Code: "ERR001", Message: "test error"},
				},
			},
			wantText: "cdek api error [ERR001]: test error",
		},
		{
			name: "single error without code",
			err: &ErrorResponse{
				Errors: []ErrorDetail{
					{Message: "test error no code"},
				},
			},
			wantText: "cdek api error: test error no code",
		},
		{
			name: "multiple errors",
			err: &ErrorResponse{
				Errors: []ErrorDetail{
					{Code: "ERR001", Message: "error 1"},
					{Code: "ERR002", Message: "error 2"},
				},
			},
			wantText: "cdek api error: 2 errors occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.wantText {
				t.Errorf("ErrorResponse.Error() = %v, want %v", got, tt.wantText)
			}
		})
	}
}

func TestParseErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
	}{
		{
			name:       "valid JSON error",
			statusCode: 400,
			body:       `{"errors":[{"code":"ERR001","message":"test error"}]}`,
			wantErr:    true,
		},
		{
			name:       "invalid JSON",
			statusCode: 500,
			body:       `invalid json`,
			wantErr:    true,
		},
		{
			name:       "empty errors array",
			statusCode: 400,
			body:       `{"errors":[]}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(bytes.NewBufferString(tt.body)),
			}

			err := parseErrorResponse(resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseErrorResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWrapHTTPError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    error
	}{
		{
			name:       "401 unauthorized",
			statusCode: 401,
			body:       `{"errors":[{"code":"UNAUTHORIZED"}]}`,
			wantErr:    ErrUnauthorized,
		},
		{
			name:       "403 forbidden",
			statusCode: 403,
			body:       `{"errors":[{"code":"FORBIDDEN"}]}`,
			wantErr:    ErrUnauthorized,
		},
		{
			name:       "404 not found",
			statusCode: 404,
			body:       `{"errors":[{"code":"NOT_FOUND"}]}`,
			wantErr:    ErrNotFound,
		},
		{
			name:       "400 bad request",
			statusCode: 400,
			body:       `{"errors":[{"code":"BAD_REQUEST"}]}`,
			wantErr:    ErrInvalidRequest,
		},
		{
			name:       "422 unprocessable",
			statusCode: 422,
			body:       `{"errors":[{"code":"INVALID"}]}`,
			wantErr:    ErrInvalidRequest,
		},
		{
			name:       "429 rate limit",
			statusCode: 429,
			body:       `{"errors":[{"code":"RATE_LIMIT"}]}`,
			wantErr:    ErrRateLimitExceeded,
		},
		{
			name:       "500 server error",
			statusCode: 500,
			body:       `{"errors":[{"code":"SERVER_ERROR"}]}`,
			wantErr:    ErrServerError,
		},
		{
			name:       "502 bad gateway",
			statusCode: 502,
			body:       `{"errors":[{"code":"BAD_GATEWAY"}]}`,
			wantErr:    ErrServerError,
		},
		{
			name:       "503 service unavailable",
			statusCode: 503,
			body:       `{"errors":[{"code":"UNAVAILABLE"}]}`,
			wantErr:    ErrServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(bytes.NewBufferString(tt.body)),
				Header:     http.Header{},
			}

			err := wrapHTTPError(resp)
			if err == nil {
				t.Fatal("wrapHTTPError() returned nil")
			}

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("wrapHTTPError() error = %v, want wrapped %v", err, tt.wantErr)
			}
		})
	}
}
