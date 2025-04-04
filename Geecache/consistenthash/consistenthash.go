package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

type Hash func(data []byte) uint32

// 封装好的一致性hash
type Map struct {
	hash     Hash
	replicas int            //虚拟节点倍数
	keys     []int          //虚拟节点的哈希值,按升序排列
	hashMap  map[int]string //虚拟节点对真实节点的映射，键是虚拟节点的哈希值，值是真实节点名称
}

// 允许自定义倍数，hash函数，默认是crc32
func New(replicas int, fn Hash) *Map {
	m := &Map{
		hash:     fn,
		replicas: replicas,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// 按名称增加节点
func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		for i := 0; i < m.replicas; i++ {
			//虚拟节点按编号区分
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			m.keys = append(m.keys, hash)
			m.hashMap[hash] = key
		}
	}
	sort.Ints(m.keys)
}

// 根据key获取真实节点名称
func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}

	hash := int(m.hash([]byte(key)))
	//二分查找，找到第一个大于等于key的虚拟节点
	idx := sort.Search(len(m.keys), func(i int) bool {
		return hash <= m.keys[i]
	})

	return m.hashMap[m.keys[idx%len(m.keys)]]
}
