package consistentHash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

// Hash :定义了函数类型 Hash，采取依赖注入的方式，默认为 crc32.ChecksumIEEE 算法，允许替换成自定义的 Hash 函数
type Hash func(data []byte) uint32

type Map struct {
	hash     Hash
	replicas int            //虚拟节点倍数
	nodes    []int          //哈希环(有序的)
	hashMap  map[int]string //虚拟节点与真实节点的映射表，键是虚拟节点的哈希值，值是真实节点的名称
}

// New : 构造函数 New() 允许自定义虚拟节点倍数和 Hash 函数
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

// Add :添加真实节点/机器，Add函数允许传入 0 或 多个真实节点的名称
func (m *Map) Add(nodeNames ...string) {
	//对每一个真实节点 key，对应创建 m.replicas 个虚拟节点
	for _, node := range nodeNames {
		for i := 0; i < m.replicas; i++ {
			//虚拟节点的名称是：strconv.Itoa(i) + node，即通过添加编号的方式区分不同虚拟节点，使用 m.hash() 计算虚拟节点的哈希值
			hash := int(m.hash([]byte(strconv.Itoa(i) + node)))
			m.nodes = append(m.nodes, hash)
			//在 hashMap 中增加虚拟节点和真实节点的映射关系
			m.hashMap[hash] = node
		}
	}
	sort.Ints(m.nodes) //环上的节点排序
}

// Get :根据key的hash值选择节点
func (m *Map) Get(key string) string {
	if len(m.nodes) == 0 {
		return ""
	}
	//第一步，计算 key 的哈希值
	hash := int(m.hash([]byte(key)))
	//第二步，顺时针找到第一个匹配的虚拟节点的下标 index
	index := sort.Search(len(m.nodes), func(i int) bool {
		return m.nodes[i] >= hash
	})
	//第三步，通过 hashMap 映射得到真实的节点，因为 m.nodes 是一个环状结构，所以用取余数的方式来处理这种情况
	return m.hashMap[m.nodes[index%len(m.nodes)]]
}
