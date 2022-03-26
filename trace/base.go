package trace

import (
	"github.com/wwqdrh/localtracing/internal/data"
)

type (
	ApiTimeHeapBuilder func(...string) (ApiTimeHeap, error)
	ApiTimeHeap        = data.IHeap
)
