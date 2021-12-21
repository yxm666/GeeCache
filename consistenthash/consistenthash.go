package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

//Hash 定义了函数类型 Hash，采取依赖注入的方式，允许用于替换称自定义的Hash函数，
//也方便测试时替换，默认为crc32.ChecksumIEE 算法
type Hash func(data []byte) uint32

//Map 一致性哈希算法的主数据结构
type Map struct {
	//Hash函数
	hash Hash
	//虚拟节点倍数
	replicas int
	//哈希环
	keys []int
	// 虚拟节点与真实节点的映射表 键是虚拟节点的哈希值，值是真实节点的名称
	hashMap map[int]string
}

//New 允许自定义虚拟节点倍数和 Hash 函数
func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

//Add 允许传入0 或者 多个真实节点的名称
func (m *Map) Add(keys ...string) {
	//对每一个真实节点 key，对应建立m.replicas 个 虚拟节点
	for _, key := range keys {
		for i := 0; i < m.replicas; i++ {
			// 虚拟节点的名称是通过编号的方式区分不同虚拟节点 	使用m.hash()计算虚拟节点的哈希值
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			m.keys = append(m.keys, hash)
			//添加到环上
			m.hashMap[hash] = key
		}
	}
	//环上的哈希值排序
	sort.Ints(m.keys)
}

//Get 实现选择节点的方法 返回真实节点的名称
func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}
	// 计算key的哈希值
	hash := int(m.hash([]byte(key)))
	//顺序找到第一个匹配的虚拟节点的下标idx，从m.keys中获取到对应的哈希值
	//sort.Search 使用二进制搜索查找并返回文献[0，n ]中 f (i)为真的最小索引 i，  n为 len(m.keys)
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})

	return m.hashMap[m.keys[idx%len(m.keys)]]
}
