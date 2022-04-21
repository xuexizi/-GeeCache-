package lru

import (
	"container/list"
	"fmt"
)

type Cache struct {
	maxBytes  int64
	nowBytes  int64
	cache     map[string]*list.Element
	ll        *list.List                    //此处链表存放的是 *Entry 类型
	onDeleted func(key string, value Value) //删除 key 值时的回调函数
}

type Entry struct {
	key   string
	value Value //此处的 value 可以是实现了 Value 接口的 任意类型
}

// Value : 可以把该接口直接看成 java 中的类，然后其他类型重写接口中的方法 Len
type Value interface {
	Len() int
}

func New(maxBytes int64, onDeleted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		onDeleted: onDeleted,
	}
}

// Add 添加键值对
func (c *Cache) Add(key string, value Value) {
	Len := c.Len()
	fmt.Println("Add调用之前，Len =", Len)

	if ele, ok := c.cache[key]; ok { //如果已经存在key，更新value
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*Entry) //将 ele.Value 转换成 *Entry 类型 并 赋值给 kv, kv是一个指针
		c.nowBytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else {
		front := c.ll.PushFront(&Entry{key, value})
		c.cache[key] = front
		c.nowBytes += int64(len(key)) + int64(value.Len())
	}

	//如果 Add 之后超出 maxBytes，删除队列中最后的元素
	for c.maxBytes != 0 && c.nowBytes > c.maxBytes {
		c.RemoveOldest()
	}

	Len = c.Len()
	fmt.Println("Add调用之后，Len =", Len)
}

// Get 根据key获取value
func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*Entry) //此处的Value是list包里的，与本文件的Value无关
		return kv.value, true
	}
	return
}

// RemoveOldest 淘汰节点
func (c *Cache) RemoveOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*Entry)
		delete(c.cache, kv.key)
		c.nowBytes -= int64(len(kv.key)) + int64(kv.value.Len())
		if c.onDeleted != nil {
			c.onDeleted(kv.key, kv.value)
		}
	}
}

// Len : 此处 Cache 实现 Value 接口的用处不大，只是单纯的得到 c.cache 中的 key 个数
func (c *Cache) Len() int {
	return c.ll.Len()
}
