package errors

import (
	"fmt"
	"runtime"
)

type ZXError struct {
	call string
	code string
	err  error
}

func (e *ZXError) Error() string {
	str := e.err.Error()
	if e.code != "" {
		str += fmt.Sprintf(" [code:%s]", e.code)
	}
	if e.call != "" {
		str += fmt.Sprintf(" at %s", e.call)
	}

	return str
}

func callinfo() string {
	_, f, l, _ := runtime.Caller(2)
	return fmt.Sprintf("%s:%d", f, l)
}
