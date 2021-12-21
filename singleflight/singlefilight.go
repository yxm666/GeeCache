package singleflight

import "sync"

//wg.Add(1) 锁加1。
//wg.Wait() 阻塞，直到锁被释放。
//wg.Done() 锁减1。
type (
	//call 代表正在进行中，或已经结束的请求 使用sync.WaitGroup 锁避免重入
	call struct {
		//WaitGroup 主要用来处理协程同步的问题 并发协程之间不需要消息传递，非常适合 sync.WaitGroup
		//并发请求的过程中，每一次请求都可以理解为一个新的协程，所以当后续有同样的请求命中时，会将后续的协程进行挂起
		wg  sync.WaitGroup
		val interface{}
		err error
	}

	//Group 是singleflight 的主数据结构 管理不同key的请求 (call)
	Group struct {
		mu sync.Mutex
		m  map[string]*call
	}
)

//Do 第一个参数是 key 第二个参数是一个函数fn
//Do的作用就是，针对相同的key,无论被调用做少次，函数fn都会被调用一次，等待fn调用结束了，返回返回值或错误
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	//Lock 是为了保护 Group 的成员变量 m 不被并发读写而加上的锁
	//后续的请求在这里会因为获得不了锁而阻塞
	g.mu.Lock()

	//对g.m延迟初始化
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	//map里有请求，挂起
	if c, ok := g.m[key]; ok {
		// 先解锁 防止
		g.mu.Unlock()
		//如果请求正在进行中，则等待
		c.wg.Wait() //进行挂起
		//请求结束，返回结果
		return c.val, c.err
	}

	c := new(call)
	//发起请求前加锁
	c.wg.Add(1)
	//添加到 g.m 表明 key 已经有对应的请求在处理 对于g.m
	g.m[key] = c //对 g.m资源进行加锁
	g.mu.Unlock()
	//调用fn() 发起请求
	c.val, c.err = fn()
	c.wg.Done() //
	// 更新 g.m
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()
	//返回结果
	return c.val, c.err

}
