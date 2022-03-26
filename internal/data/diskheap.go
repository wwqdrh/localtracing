package data

import (
	"fmt"
	"os"
	"strconv"
	"sync/atomic"

	"github.com/wwqdrh/localtracing/internal/driver"
)

// 使用leveldb存储数据而不是直接加在内存中
// 堆的名字-len: 维护总值
// 堆的名字-索引位置: 值
const noneInt = -1 << 31

var defaultDriver *driver.LevelDBDriver

type diskHeap struct {
	length   int64 // 维护一个内存中的length，减少调用 分析可知Len耗时最多
	d        *driver.LevelDBDriver
	heapName string
	dbName   string
}

func init() {
	driver, err := driver.NewLevelDBDriver("./temp/heap")
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}
	defaultDriver = driver
}

//  prefix, dbName, heapName string
func NewDiskHeap(args ...string) (IHeap, error) {
	// heap.Push()
	_, dbName, heapName := args[0], args[1], args[2]

	return &diskHeap{
		length:   -1,
		d:        defaultDriver,
		heapName: heapName,
		dbName:   dbName,
	}, nil
}

func (h *diskHeap) lenKey() string {
	return h.heapName + "@len"
}

func (h *diskHeap) idKey(i int) string {
	return fmt.Sprintf("%s@id=%d", h.heapName, i)
}

// -1<<31 就是不存在
func (h *diskHeap) getVal(i int) int {
	var a int
	if data, err := h.d.Get(h.dbName, h.idKey(i)); err != nil {
		return noneInt
	} else {
		v, err := strconv.ParseInt(string(data), 10, 64)
		if err != nil {
			return noneInt
		}
		a = int(v)
	}
	return a
}

func (h *diskHeap) Len() int {
	if h.length != -1 {
		return int(h.length)
	}

	if data, err := h.d.Get(h.dbName, h.lenKey()); err != nil {
		atomic.CompareAndSwapInt64(&h.length, -1, 0)
	} else {
		v, err := strconv.ParseInt(string(data), 10, 64)
		if err != nil {
			atomic.CompareAndSwapInt64(&h.length, -1, 0)
		}
		atomic.CompareAndSwapInt64(&h.length, -1, v)
	}
	return int(h.length)
}

func (h *diskHeap) Less(i, j int) bool {
	a, b := h.getVal(i), h.getVal(j)
	if a == noneInt || b == noneInt {
		return false
	}

	return a < b
}

// TODO 目前的swap是非原子的，可能存在数据不一致
func (h *diskHeap) Swap(i, j int) {
	var a, b []byte
	if data, err := h.d.Get(h.dbName, h.idKey(i)); err != nil {
		return
	} else {
		a = data
	}
	if data, err := h.d.Get(h.dbName, h.idKey(j)); err != nil {
		return
	} else {
		b = data
	}
	h.d.Put(h.dbName, h.idKey(i), b)
	h.d.Put(h.dbName, h.idKey(j), a)
}

func (h *diskHeap) Push(val interface{}) {
	h.d.Put(h.dbName, h.idKey(h.Len()), []byte(fmt.Sprint(val.(int))))
	h.d.Put(h.dbName, h.lenKey(), []byte(fmt.Sprint(h.Len()+1)))
	atomic.AddInt64(&h.length, 1)
}

func (h *diskHeap) Pop() interface{} {
	res := h.getVal(h.Len() - 1)
	if res == noneInt {
		return nil
	}
	h.d.Put(h.dbName, h.lenKey(), []byte(fmt.Sprint(h.Len()-1)))
	atomic.AddInt64(&h.length, -1)
	return res
}

func (h *diskHeap) Truncate() error {
	h.length = -1
	return h.d.Truncate(h.dbName)
}

func (h *diskHeap) Top() interface{} {
	if h.Len() == 0 {
		return nil
	}
	return h.getVal(0)
}
