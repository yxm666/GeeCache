package geecache

//是
//接收 key --> 检查是否被缓存 -----> 返回缓存值 ⑴
//|  否                         是
//|-----> 是否应当从远程节点获取 -----> 与远程节点交互 --> 返回缓存值 ⑵
//|  否
//|-----> 调用`回调函数`，获取值并添加到缓存 --> 返回缓存值 ⑶
// 实例化lru 封装get 和 add方法，并添加互斥锁 mu
import (
	"GeeCache/lru"
	"sync"
)

//实例化lru，封装get 和 add 方法，并添加互斥锁 mu
type cache struct {
	mu sync.Mutex
	// 对 LRU进行封装 增加互斥锁字段
	lru        *lru.Cache
	cacheBytes int64
}

func (c *cache) add(key string, value ByteView) {
	// 加锁
	c.mu.Lock()
	// 最后执行 defer
	defer c.mu.Unlock()
	//延迟初始化(Lazy Initialization) 一个对象延迟初始化意味这该对象的创建将会延迟至第一次使用该对象时。主要用于提高性能，并减少程序内存要求。
	if c.lru == nil {
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)
}

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
