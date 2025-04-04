package singleflight

import "sync"

//可以避免缓存击穿，但是不能避免缓存穿透
//并发控制机制，对于相同的键，同一时间只有一个操作在执行，其他并发会等待该操作完成并共享结果

// 某个请求的结果
type call struct {
	wg  sync.WaitGroup //用于等待正在执行的函数完成
	val interface{}
	err error
}

// 管理所有并发请求
type Group struct {
	mu sync.Mutex
	m  map[string]*call
}

func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	//检查key是否存在
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait() //存在等待并返回已有结果
		return c.val, c.err
	}

	//不存在就新建call并添加到map
	c := new(call)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	//执行函数并保存结果
	c.val, c.err = fn()
	c.wg.Done()

	//清理map中的key
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}
