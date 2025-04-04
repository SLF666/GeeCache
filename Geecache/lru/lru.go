package lru

import "container/list"

// LRU 缓存淘汰算法，具体处理缓存的增删改查
// 最近最少使用，用一个队列保存缓存数据，每次访问的数据放到队首，当做最近访问，从队尾开始删除
type Cache struct {
	maxBytes  int64 //缓存最大容量
	nbytes    int64 //当前已使用缓存
	ll        *list.List
	cache     map[string]*list.Element
	OnEvicted func(key string, value Value) //回调函数，当某个元素被删除时调用
	//这种设计允许你在元素被从缓存中删除时执行一些自定义逻辑，比如记录日志、更新其他数据结构或进行任何必要的清理工作
}

// 链表节点的元素，值可以是实现了Value接口的任意类型
type entry struct {
	key   string
	value Value
}

// 对于只有Ascll码的字符串，一个字符占一个字节
type Value interface {
	Len() int
}

// 构造函数，参数为最大容量和回调函数
func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEvicted,
	}
}

// 查找元素，找到就移动到队首，并返回
func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		//将ele断言成entry类型
		kv := ele.Value.(*entry) //这里的Value是*list.Element的一个字段，类型是any所以要断言，不是上面自己写的Value接口
		return kv.value, true
	}
	return
}

// 删除，淘汰队尾缓存
func (c *Cache) RemoveOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key)
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// 新增&修改
func (c *Cache) Add(key string, value Value) {
	//如果存在，更新
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else {
		ele := c.ll.PushFront(&entry{key, value})
		c.cache[key] = ele
		c.nbytes += int64(len(key)) + int64(value.Len())
	}

	//如果超出限制，淘汰
	for c.maxBytes != 0 && c.maxBytes < c.nbytes {
		c.RemoveOldest()
	}
}

// 当前存储的数据个数
func (c *Cache) Len() int {
	return c.ll.Len()
}

// 错误，类型形参不能单独使用
type CommonType[T int | string | float32] []T
