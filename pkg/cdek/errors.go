package cdek

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Стандартные ошибки CDEK API
var (
	ErrNotFound          = fmt.Errorf("cdek: not found")
	ErrUnauthorized      = fmt.Errorf("cdek: unauthorized")
	ErrInvalidRequest    = fmt.Errorf("cdek: invalid request")
	ErrRateLimitExceeded = fmt.Errorf("cdek: rate limit exceeded")
	ErrServerError       = fmt.Errorf("cdek: server error")
)

// ErrorResponse структура ошибки от CDEK API
type ErrorResponse struct {
	Errors   []ErrorDetail  `json:"errors,omitempty"`
	Requests []RequestState `json:"requests,omitempty"`
}

// RequestState is a CDEK v2 order response request entry.
type RequestState struct {
	State  string        `json:"state,omitempty"`
	Errors []ErrorDetail `json:"errors,omitempty"`
}

// ErrorDetail детали ошибки
type ErrorDetail struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// allErrors returns top-level errors merged with nested request errors.
func (e *ErrorResponse) allErrors() []ErrorDetail {
	var all []ErrorDetail
	all = append(all, e.Errors...)
	for _, r := range e.Requests {
		all = append(all, r.Errors...)
	}
	return all
}

// Error реализует интерфейс error
func (e *ErrorResponse) Error() string {
	errs := e.allErrors()
	if len(errs) == 0 {
		return "cdek api error: unknown error"
	}

	if len(errs) == 1 {
		err := errs[0]
		if err.Code != "" {
			return fmt.Sprintf("cdek api error [%s]: %s", err.Code, err.Message)
		}
		return fmt.Sprintf("cdek api error: %s", err.Message)
	}

	return fmt.Sprintf("cdek api error: %d errors occurred", len(errs))
}

// parseErrorResponse парсит HTTP ответ как ошибку
func parseErrorResponse(resp *http.Response) error {
	defer func() {
		_ = resp.Body.Close() // Игнорируем ошибку закрытия
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read error response: %w", err)
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		// Если не удалось распарсить как JSON, возвращаем текст как есть
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}

	if len(errResp.allErrors()) == 0 {
		return fmt.Errorf("http %d: unknown error", resp.StatusCode)
	}

	return &errResp
}

// wrapHTTPError оборачивает HTTP ошибку в типизированную
func wrapHTTPError(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%w: %s", ErrUnauthorized, parseErrorResponse(resp))
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, parseErrorResponse(resp))
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return fmt.Errorf("%w: %s", ErrInvalidRequest, parseErrorResponse(resp))
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", ErrRateLimitExceeded, parseErrorResponse(resp))
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return fmt.Errorf("%w: %s", ErrServerError, parseErrorResponse(resp))
	default:
		return parseErrorResponse(resp)
	}
}
