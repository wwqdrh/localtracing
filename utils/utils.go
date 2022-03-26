package utils

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

func GoID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func DirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

// func GetHeapMemory(h heap.Interface) int64 {
// 	QSizeStr := fmt.Sprint(uintptr(len(h.IntSlice)) * reflect.TypeOf(h.IntSlice).Elem().Size())
// 	QSize, _ := strconv.ParseInt(QSizeStr, 10, 64)
// 	return QSize
// }
