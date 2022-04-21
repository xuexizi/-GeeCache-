package geecache

import (
	pb "GeeCache/geecache/geeCachePB"
	"GeeCache/geecache/singleFlight"
	"fmt"
	"log"
	"sync"
)

// Group 是一个缓存的命名空间， 并且关联数据
type Group struct {
	name      string              //一个 Group 可以认为是一个缓存的命名空间，每个 Group 拥有一个唯一的名称 name。比如可以创建三个 Group，缓存学生的成绩命名为 scores，缓存学生信息的命名为 info，缓存学生课程的命名为 courses
	getter    Getter              //缓存未命中时获取源数据的回调(callback)
	mainCache cache               //并发缓存
	nodes     NodePicker          //根据传入的 key 选择相应节点 NodeGetter
	loader    *singleFlight.Group //确保每个key只被调用一次
}

// Getter : 定义接口 Getter 和 回调函数 Get(key string)([]byte, error)，参数是 key，返回值是 []byte
type Getter interface {
	Get(key string) ([]byte, error)
}

// GetterFunc : 定义函数类型 GetterFunc，并实现 Getter 接口的 Get 方法。
type GetterFunc func(key string) ([]byte, error)

// Get : 函数类型实现某一个接口，称之为接口型函数，方便使用者在调用时既能够传入函数作为参数，也能够传入实现了该接口的结构体作为参数
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

// NewGroup : 构建函数，用来实例化 Group，并且将 group 存储在全局变量 groups 中
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		loader:    &singleFlight.Group{},
	}
	groups[name] = g
	return g
}

//GetGroup 用来查询特定名称的 Group，这里使用了只读锁 RLock()，因为不涉及任何冲突变量的写操作
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

/*
	Get : GeeCache 最为核心的方法
		流程 1 ：从 mainCache 中查找缓存，如果存在则返回缓存值
				如果不存在，则调用 load 方法。
		流程 2 ：使用 PickNode 方法选择节点，若非本机节点，则调用 getFromNode 从远程获取。
		流程 3 ：若是本机节点或失败，则回退到 getLocally，然后通过 调用用户的回调函数 g.getter.Get() 获取源数据
*/
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}
	if v, ok := g.mainCache.get(key); ok {
		return v, nil
	}
	return g.load(key) //缓存不存在,去数据库里查询
}

//RegisterNodes :  将 实现了 NodePicker 接口的 HTTPPool 注入到 Group 中
func (g *Group) RegisterNodes(nodes NodePicker) {
	if g.nodes != nil {
		panic("RegisterNodePicker 调用多次")
	}
	g.nodes = nodes
}

//load : 使用 PickNode 方法选择节点，若非本机节点，则调用 getFromNode 从远程获取。若是本机节点或失败，则回退到 getLocally
func (g *Group) load(key string) (value ByteView, err error) {
	//将原来的 load 的逻辑，使用 g.loader.Do 包裹起来即可，这样确保了并发场景下针对相同的 key，load 过程只会调用一次
	do, doErr := g.loader.Do(key, func() (interface{}, error) {
		if g.nodes != nil {
			if node, ok := g.nodes.PickNode(key); ok {
				if value, err = g.getFromNode(node, key); err == nil {
					return value, nil
				}
				log.Println("geeCache获取远程节点失败", err)
			}
		}
		return g.getLocally(key)
	})

	if doErr == nil {
		return do.(ByteView), doErr
	}
	return
}

//getFromNode : 使用实现了 NodeGetter 接口的 httpGetter 访问远程节点，获取缓存值
func (g *Group) getFromNode(node NodeGetter, key string) (ByteView, error) {
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	err := node.Get(req, res)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{bv: res.Value}, nil
}

//getLocally 调用用户回调函数 g.getter.Get() 获取源数据，并将源数据添加到缓存 mainCache 中（通过 populateCache 方法）
func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	value := ByteView{bv: cloneBytes(bytes)}
	g.populateCache(key, value)
	return value, nil
}

// 将源数据添加到缓存 mainCache 中
func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}
