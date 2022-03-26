package data

import (
	"sort"
)

type MemoryHeap struct {
	sort.IntSlice
}

func NewMemoryHeap(...string) (IHeap, error) {
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
