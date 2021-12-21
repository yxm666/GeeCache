package lru

import "container/list"

//Cache LRU的实现算法是基于一个map 与 一个 队列，map中的value 为Queue的值，使用Golang的双向链表来实现队列
type (
	Cache struct {
		//允许使用的最大内存
		maxBytes int64
		//当前已使用的内存
		nbytes int64
		// list.list 表示一个双向链表
		ll *list.List
		//键是字符串，值是双线链表中对应的指针
		cache map[string]*list.Element
		//某条记录被移除时的回调函数，可以为nil
		onEvicted func(key string, value Value)
	}

	//entry 是双向链表节点中的数据类型，在链表中仍保存每个值对应的key的好处在于，淘汰队首节点时，需要用key从字典中删除对应的映射
	entry struct {
		key string
		// 值是任意实现了 Value接口的类型
		value Value
	}

	//Value 为了通用性，我们允许值是实现了 Value接口的任意类型，该接口只包含了一个方法 Len()，用于返回值所占用的内存大小
	Value interface {
		Len() int
	}
)

func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		onEvicted: onEvicted,
	}
}

//Get 查找主要有2个步骤，第一步是从字典中找到对应的双向链表节点，第二步，将该节点移动到队尾
func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := c.cache[key]; ok {
		// 移动到队列的对尾 （将链表中的节点 ele 移动到队尾 【双向链表作为队列，队首队尾是相对的，在这里约定 front 为队尾】）
		c.ll.MoveToFront(ele)
		//ele.Value 是 一个 Interface{}
		kv := ele.Value.(*entry)
		//如果键对应的链表节点存在，则将对应节点移动到队尾，并返回查找到的值
		return kv.value, true
	}
	// 默认返回默认值 nil false
	return
}

func (c *Cache) RemoveOldest() {
	//取到队首节点，从链表中删除
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		// interface 断言
		kv := ele.Value.(*entry)
		// 按照节点的key删除 map中对应的映射
		delete(c.cache, kv.key)
		//更新当前所用的内存  存的是 entry：key + value的大小一块减少
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		//如果回调函数 OnEvicted 不为 nil，则调用回调函数
		// 被淘汰的时候调用回调函数 进行额外处理
		if c.onEvicted != nil {
			c.onEvicted(kv.key, kv.value)
		}
	}
}

func (c *Cache) Add(key string, value Value) {
	if ele, ok := c.cache[key]; ok {
		// 如果键存在 则更新对应节点的值，并将该节点移到队尾(链表的头)
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else {
		//新增的键
		ele := c.ll.PushFront(&entry{key, value})
		c.cache[key] = ele
		c.nbytes += int64(len(key)) + int64(value.Len())
	}
	// 更新 c.nbytes 如果超过了设定的最大值c.maxBytes 则移除访问最少的节点
	// 更新 跟 添加都会造成淘汰 更新键值对大的时候会淘汰
	for c.maxBytes != 0 && c.maxBytes < c.nbytes {
		c.RemoveOldest()
	}
}

// Len the number of cache entries
func (c *Cache) Len() int {
	return c.ll.Len()
}
