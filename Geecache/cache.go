package geecache

//并发控制缓存，封装好的缓存对象
import (
	"geecache/lru"
	"sync"
)

// 真正的缓存数据结构，包含一个锁，具体处理缓存的对象，缓存最大容量
type cache struct {
	mu         sync.Mutex
	lru        *lru.Cache
	cacheBytes int64
}

// 封装好的添加缓存的方法
func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)
}

// 获取缓存，返回值是ByteView，和bool值，表示是否获取成功
func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		return
	}

	if v, ok := c.lru.Get(key); ok {
		return v.(ByteView), ok
	}

	return
}
