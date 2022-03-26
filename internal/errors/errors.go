package errors

import (
	"errors"
	"fmt"
)

const (
	ErrCrash     ErrorType = "crash"     // 宕机错误
	ErrNetwork   ErrorType = "network"   // 网络错误
	ErrRequest   ErrorType = "request"   // 请求错误，参数不对或接口不存在
	ErrBussiness ErrorType = "bussiness" // 业务错误，业务逻辑中出现某些异常
	ErrPage      ErrorType = "page"      // 页面错误，前端js报错时推送的
)

type ErrorType string

func (t ErrorType) WithID(id interface{}) string {
	return fmt.Sprintf("%s_%v", t, id)
}

func New(msg string) error {
	return &ZXError{call: callinfo(), err: errors.New(msg)}
}

func NewCode(c, msg string) error {
	return &ZXError{call: callinfo(), code: c, err: errors.New(msg)}
}

func WithCode(c string, err error) error {
	return &ZXError{call: callinfo(), err: err}
}
