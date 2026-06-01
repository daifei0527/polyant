// pkg/polysdk/errors.go
package polysdk

import "fmt"

// Error Polyant SDK 错误
type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("polyant error %d: %s", e.Code, e.Message)
}

// NewError 创建错误
func NewError(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

// IsNotFoundError 判断是否为未找到错误
func IsNotFoundError(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == 404
	}
	return false
}

// IsAuthError 判断是否为认证错误
func IsAuthError(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == 401 || e.Code == 403
	}
	return false
}
