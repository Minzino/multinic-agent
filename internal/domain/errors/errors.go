package errors

import (
	"errors"
	"fmt"
)

// ErrorType은 에러의 종류를 나타냅니다
type ErrorType string

const (
	// ErrorTypeValidation은 유효성 검증 실패를 나타냅니다
	ErrorTypeValidation ErrorType = "VALIDATION"
	
	// ErrorTypeNotFound는 리소스를 찾을 수 없음을 나타냅니다
	ErrorTypeNotFound ErrorType = "NOT_FOUND"
	
	// ErrorTypeConflict는 충돌이 발생했음을 나타냅니다
	ErrorTypeConflict ErrorType = "CONFLICT"
	
	// ErrorTypeSystem은 시스템 레벨 에러를 나타냅니다
	ErrorTypeSystem ErrorType = "SYSTEM"
	
	// ErrorTypeNetwork는 네트워크 관련 에러를 나타냅니다
	ErrorTypeNetwork ErrorType = "NETWORK"
	
	// ErrorTypeTimeout은 타임아웃 에러를 나타냅니다
	ErrorTypeTimeout ErrorType = "TIMEOUT"
)

// DomainError는 도메인 레벨의 에러를 나타냅니다
type DomainError struct {
	Type    ErrorType
	Message string
	Cause   error
}

// Error는 error 인터페이스를 구현합니다
func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap은 내부 에러를 반환합니다
func (e *DomainError) Unwrap() error {
	return e.Cause
}

// Is는 에러 비교를 위한 메서드입니다
func (e *DomainError) Is(target error) bool {
	t, ok := target.(*DomainError)
	if !ok {
		return false
	}
	return e.Type == t.Type
}

// 생성자 함수들

// NewValidationError는 유효성 검증 에러를 생성합니다
func NewValidationError(message string, cause error) *DomainError {
	return &DomainError{
		Type:    ErrorTypeValidation,
		Message: message,
		Cause:   cause,
	}
}

// NewNotFoundError는 리소스를 찾을 수 없는 에러를 생성합니다
func NewNotFoundError(message string) *DomainError {
	return &DomainError{
		Type:    ErrorTypeNotFound,
		Message: message,
	}
}

// NewConflictError는 충돌 에러를 생성합니다
func NewConflictError(message string) *DomainError {
	return &DomainError{
		Type:    ErrorTypeConflict,
		Message: message,
	}
}

// NewSystemError는 시스템 에러를 생성합니다
func NewSystemError(message string, cause error) *DomainError {
	return &DomainError{
		Type:    ErrorTypeSystem,
		Message: message,
		Cause:   cause,
	}
}

// NewNetworkError는 네트워크 관련 에러를 생성합니다
func NewNetworkError(message string, cause error) *DomainError {
	return &DomainError{
		Type:    ErrorTypeNetwork,
		Message: message,
		Cause:   cause,
	}
}

// NewTimeoutError는 타임아웃 에러를 생성합니다
func NewTimeoutError(message string) *DomainError {
	return &DomainError{
		Type:    ErrorTypeTimeout,
		Message: message,
	}
}

// 에러 타입 확인 헬퍼 함수들

// IsValidationError는 유효성 검증 에러인지 확인합니다
func IsValidationError(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type == ErrorTypeValidation
	}
	return false
}

// IsNotFoundError는 리소스를 찾을 수 없는 에러인지 확인합니다
func IsNotFoundError(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type == ErrorTypeNotFound
	}
	return false
}

// IsSystemError는 시스템 에러인지 확인합니다
func IsSystemError(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type == ErrorTypeSystem
	}
	return false
}

// IsNetworkError는 네트워크 에러인지 확인합니다
func IsNetworkError(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type == ErrorTypeNetwork
	}
	return false
}

// IsTimeoutError는 타임아웃 에러인지 확인합니다
func IsTimeoutError(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type == ErrorTypeTimeout
	}
	return false
}