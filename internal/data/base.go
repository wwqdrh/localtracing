package data

import "container/heap"

type IHeap interface {
	heap.Interface
	Top() interface{}
}
