package cloudstorage

import (
	"fmt"
	"net/http"
	"testing"
)

func TestUploadError_Retryable_NetworkError(t *testing.T) {
	err := &UploadError{StatusCode: 0, Err: fmt.Errorf("connection refused"), Message: "network/timeout"}
	if !err.Retryable() {
		t.Error("сетевая ошибка должна быть retryable")
	}
}

func TestUploadError_Retryable_5xx(t *testing.T) {
	for _, code := range []int{500, 502, 503, 504} {
		err := &UploadError{StatusCode: code, Message: "server error"}
		if !err.Retryable() {
			t.Errorf("HTTP %d должен быть retryable", code)
		}
	}
}

func TestUploadError_Retryable_429(t *testing.T) {
	err := &UploadError{StatusCode: http.StatusTooManyRequests, Message: "rate limit"}
	if !err.Retryable() {
		t.Error("429 должен быть retryable")
	}
}

func TestUploadError_NotRetryable_4xx(t *testing.T) {
	for _, code := range []int{400, 401, 403, 404, 409} {
		err := &UploadError{StatusCode: code, Message: "client error"}
		if err.Retryable() {
			t.Errorf("HTTP %d НЕ должен быть retryable", code)
		}
	}
}

func TestIsRetryable_NilError(t *testing.T) {
	if IsRetryable(nil) {
		t.Error("nil ошибка не должна быть retryable")
	}
}

func TestIsRetryable_UploadError(t *testing.T) {
	err := &UploadError{StatusCode: 503}
	if !IsRetryable(err) {
		t.Error("503 UploadError должен быть retryable")
	}
}

func TestIsRetryable_WrappedUploadError(t *testing.T) {
	inner := &UploadError{StatusCode: 403}
	wrapped := fmt.Errorf("get upload URL: %w", inner)
	if IsRetryable(wrapped) {
		t.Error("обёрнутая 403 UploadError НЕ должна быть retryable")
	}
}

func TestIsRetryable_PlainError(t *testing.T) {
	err := fmt.Errorf("unknown error")
	if !IsRetryable(err) {
		t.Error("обычная ошибка должна быть retryable (на всякий случай)")
	}
}

func TestUploadError_Error_WithStatusCode(t *testing.T) {
	err := &UploadError{StatusCode: 500, Message: "internal"}
	got := err.Error()
	if got != "HTTP 500: internal" {
		t.Errorf("got %q, want %q", got, "HTTP 500: internal")
	}
}

func TestUploadError_Error_NetworkError(t *testing.T) {
	inner := fmt.Errorf("dial tcp: timeout")
	err := &UploadError{Err: inner, Message: "network"}
	got := err.Error()
	if got != "dial tcp: timeout" {
		t.Errorf("got %q, want %q", got, "dial tcp: timeout")
	}
}

func TestUploadError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("original")
	err := &UploadError{Err: inner}
	if err.Unwrap() != inner {
		t.Error("Unwrap должен вернуть оригинальную ошибку")
	}
}
