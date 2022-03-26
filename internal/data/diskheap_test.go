package data

import (
	"container/heap"
	"fmt"
	"os"
	"testing"
)

func TestDiskHeap(t *testing.T) {
	h, err := NewDiskHeap("./temp/heap", "diskheap", "heap1")
	if err != nil {
		t.Error(err)
	}
	defer func() {
		os.RemoveAll("./temp/heap")
	}()

	fmt.Println(h.Len())

	heap.Push(h, 1)
	heap.Push(h, 2)
	heap.Push(h, 3)
	if h.Len() != 3 {
		t.Error("长度不对")
	}

	if heap.Pop(h).(int) != 1 {
		t.Error("堆顶不对")
	}

	if h.Len() != 2 {
		t.Error("长度不对")
	}

	if heap.Pop(h).(int) != 2 {
		t.Error("堆顶不对")
	}

	if h.Len() != 1 {
		t.Error("长度不对")
	}
}

func TestDiskHeap2(t *testing.T) {
	h, err := NewDiskHeap("./temp/heap", "diskheap", "heap2")
	if err != nil {
		t.Error(err)
	}
	defer func() {
		os.RemoveAll("./temp/heap")
	}()
	fmt.Println(h.Len())

	heap.Push(h, -11)
	heap.Push(h, -2)
	heap.Push(h, -3)
	if h.Len() != 3 {
		t.Error("长度不对")
	}

	if heap.Pop(h).(int) != -11 {
		t.Error("堆顶不对")
	}

	if h.Len() != 2 {
		t.Error("长度不对")
	}

	if heap.Pop(h).(int) != -3 {
		t.Error("堆顶不对")
	}

	if h.Len() != 1 {
		t.Error("长度不对")
	}
}
