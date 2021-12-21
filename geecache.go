package geecache

import (
	pb "GeeCache/geecachepb"
	"GeeCache/singleflight"
	"fmt"
	"log"
	"sync"
)

/**
设计一个回调函数(callback)，在缓存不存在时，调用这个函数，得到源数据

缓存获取逻辑：
                           是
接收 key --> 检查是否被缓存 -----> 返回缓存值 ⑴
                |  否                         是
                |-----> 是否应当从远程节点获取 -----> 与远程节点交互 --> 返回缓存值 ⑵
                            |  否
                            |-----> 调用`回调函数`，获取值并添加到缓存 --> 返回缓存值 ⑶
*/

type Getter interface {
	Get(key string) ([]byte, error)
}

type GetterFunc func(key string) ([]byte, error)

//Get 实现Getter 接口，函数类型实现某一个接口，称为接口型函数，方便使用者在调用时既能够传入函数作为参数，
//也能够传入实现了该接口的结构体作为参数
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// Group 核心数据结构 缓存的命名空间
type Group struct {
	// Group 拥有唯一的名称 name
	name string
	// 缓存未命中时获取源数据的回调(callback)
	getter Getter
	// 并发缓存
	mainCache cache
	//新增选择节点的变量 对于Group来说 每一个peers 是一个HTTPPool
	peers PeerPicker
	//对于每一个key只会匹配一次
	loader *singleflight.Group
}

var (
	mu sync.RWMutex
	//全局变量 groups 存放多个 缓存
	groups = make(map[string]*Group)
)

func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	// 如果没有回调函数 则报错
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	// 实例化缓存
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		loader:    &singleflight.Group{},
	}
	// 将 group 存储在 全局变量 groups 中
	groups[name] = g
	return g
}

func GetGroup(name string) *Group {
	// 只读锁 不涉及任何冲突变量的写操作
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

//Get 回调函数 参数是 key 返回是 []byte
/*
	Get方法实现了开头的流程(1) 命中缓存和 流程（3）使用回调函数 获取值并添加到缓存
*/
func (g *Group) Get(key string) (ByteView, error) {
	// 如果没有key，则返回空的传 并传出报错
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	// 在缓存中获取了缓存 并命中缓存
	if v, ok := g.mainCache.get(key); ok {
		log.Println("[GeeCache] hit")
		// 将命中的缓存进行获取
		return v, nil
	}
	// 没有命中缓存 mainCache中没有 对于的key value 则调用 load方法
	// load 会调用 getLocally（分布式场景下会调用 getFromPeer 从其他节点获取），
	//getLocally 调用用户回调函数 g.getter.Get（）获取源数据

	return g.load(key)
}

// 优先从 远程节点获取缓存值，如果远程节点中无法获得缓存值，则从本地节点中获取节点
func (g *Group) load(key string) (value ByteView, err error) {
	//将原来的 load 的逻辑，使用g.loader.Do 包裹起来 确保了并发场景下针对相同的 key,load 过程只会调用一次
	viewi, err := g.loader.Do(key, func() (interface{}, error) {
		if g.peers != nil {
			//使用 PickPeer() 方法选择节点
			if peer, ok := g.peers.PickPeer(key); ok {
				//若非本机节点 则调用 getFromPeer() 从远程后去
				if value, err = g.getFromPeer(peer, key); err == nil {
					return value, nil
				}
				log.Println("[GeeCache] Failed to get from peer", err)
			}
		}
		//若是本机节点或失败 则退回到 getLocally()
		return g.getLocally(key)
	})

	if err == nil {
		return viewi.(ByteView), err
	}

	return

}

//getFromPeer 使用实现了 PeerGetter 接口的 httpGetter 从访问远程节点，获取缓存值
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	//构造pb请求
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	err := peer.Get(req, res)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: res.Value}, nil
}

func (g *Group) getLocally(key string) (value ByteView, err error) {
	//获取数据源
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	//从数据源中获取值 进行拷贝（only read）
	value = ByteView{b: cloneBytes(bytes)}
	// 将对应的键值添加到 mainCache 中
	g.populateCache(key, value)
	return value, nil
}
func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}

//RegisterPeers 实现了 PeerPicker 接口的 HTTPPool 注入到 Group 中
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	//HTTPPool实现了 PeerPicker接口 所以传入的是HTTPPool
	g.peers = peers
}
