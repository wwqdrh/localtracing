package localtracing

import "container/heap"

type (
	ApiTimeHeapInterface interface {
		heap.Interface
		Top() interface{}
		Truncate() error // 清空存储
		GetSize() int64
	}

	ApiTimeHeapBuilder func(...string) (ApiTimeHeapInterface, error)
)
