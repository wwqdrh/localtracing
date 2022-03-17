package localtracing

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
)

type MemoryHeap struct {
	sort.IntSlice
}

func NewMemoryHeap(...string) (ApiTimeHeapInterface, error) {
	return &MemoryHeap{}, nil
}

func (h *MemoryHeap) Top() interface{} {
	if len(h.IntSlice) == 0 {
		return nil
	}
	return h.IntSlice[0]
}
func (h *MemoryHeap) Push(val interface{}) { h.IntSlice = append(h.IntSlice, val.(int)) }
func (h *MemoryHeap) Pop() interface{} {
	res := h.IntSlice[len(h.IntSlice)-1]
	h.IntSlice = h.IntSlice[:len(h.IntSlice)-1]
	return res
}

func (h *MemoryHeap) Truncate() error {
	h.IntSlice = h.IntSlice[:0]
	return nil
}

func (h *MemoryHeap) GetSize() int64 {
	QSizeStr := fmt.Sprint(uintptr(len(h.IntSlice)) * reflect.TypeOf(h.IntSlice).Elem().Size())
	QSize, _ := strconv.ParseInt(QSizeStr, 10, 64)
	return QSize
}
