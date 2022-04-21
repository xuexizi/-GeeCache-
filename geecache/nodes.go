package geecache

import pb "GeeCache/geecache/geeCachePB"

//NodePicker 的 PickNode() 方法用于根据传入的 key 选择相应节点 NodeGetter
type NodePicker interface {
	PickNode(key string) (node NodeGetter, ok bool)
}

//NodeGetter 的 Get() 方法用于从对应 group 查找缓存值。NodeGetter 就对应于 HTTP 客户端
type NodeGetter interface {
	Get(in *pb.Request, out *pb.Response) error
}
