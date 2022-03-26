package cache

import (
	"container/list"
	"errors"
	"sync"
)

type LruCache struct {
	capacity int // 最大容量
	data     *list.List
	mapping  map[interface{}]*list.Element
	mu       *sync.Mutex // 并发安全
}

type node struct {
	Key interface{}
	Val interface{}
}

func NewLruCache(len int) *LruCache {
	return &LruCache{
		capacity: len,
		data:     list.New(),
		mapping:  make(map[interface{}]*list.Element),
		mu:       new(sync.Mutex),
	}
}

// 添加元素
func (c *LruCache) Add(key interface{}, val interface{}) error {
	if c.data == nil {
		return errors.New("未初始化")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.mapping[key]; ok {
		e.Value.(*node).Val = val
		c.data.MoveToFront(e)
		return nil
	}

	ele := c.data.PushFront(&node{
		Key: key,
		Val: val,
	})
	c.mapping[key] = ele
	if c.capacity != 0 && c.data.Len() > c.capacity {
		if e := c.data.Back(); e != nil {
			c.data.Remove(e)
			node := e.Value.(*node)
			delete(c.mapping, node.Key)
		}
	}
	return nil
}

func (c *LruCache) Get(key interface{}) (interface{}, bool) {
	if c.data == nil {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if ele, ok := c.mapping[key]; ok {
		c.data.MoveToFront(ele)
		return ele.Value.(*node).Val, true
	}
	return nil, false
}

func (c *LruCache) Remove(key interface{}) error {
	if ele, ok := c.mapping[key]; ok {
		c.data.Remove(ele)
		delete(c.mapping, key)
	}
	return nil
}

func (c *LruCache) Len() int {
	return c.data.Len()
}
