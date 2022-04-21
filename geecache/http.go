package geecache

import (
	"GeeCache/geecache/consistentHash"
	pb "GeeCache/geecache/geeCachePB"
	"fmt"
	"github.com/golang/protobuf/proto"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

const (
	defaultBasePath = "/_geeCache/"
	defaultReplicas = 50
)

//创建具体的 HTTP 客户端类 httpGetter，实现 NodeGetter 接口
type httpGetter struct {
	baseURL string //表示将要访问的远程节点的地址，例如 http://example.com/_geecache/
}

func (h *httpGetter) Get(in *pb.Request, out *pb.Response) error {
	u := fmt.Sprintf("%v%v/%v", h.baseURL, url.QueryEscape(in.GetGroup()), url.QueryEscape(in.GetKey()))

	//使用 http.Get() 方式获取返回值，并转换为 []bytes 类型
	res, err := http.Get(u)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("res.StatusCode != 200, server returned: %v", res.Status)
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("ioutil.ReadAll(res.Body) 失败, response body : %v", err)
	}

	//Get() 中使用 proto.Unmarshal() 解码 HTTP 响应
	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("proto.Unmarshal 失败, decoding response body : %v", err)
	}
	return nil
}

// HTTPPool 作为一个 HTTP 的节点池，实现了 NodePicker 接口和 Handler 接口
type HTTPPool struct {
	self        string                 //用来记录自己的地址，包括主机名/IP 和端口
	basePath    string                 //节点间通讯地址的前缀
	mu          sync.Mutex             //保护 nodes 和 httpGetters
	nodes       *consistentHash.Map    //根据具体的 key 选择节点
	httpGetters map[string]*httpGetter //映射远程节点与对应的 httpGetter，因为一个 httpGetter 对应一个远程节点地址 baseURL
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

// ServeHTTP : 处理所有的 http 请求
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//首先判断访问路径的前缀是否是 basePath
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	p.Log("%s %s", r.Method, r.URL.Path)

	// 约定访问路径格式为 /<basepath>/<groupname>/<key>, 通过 groupname 得到 group 实例，再使用 group.Get(key) 获取缓存数据。
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	groupName := parts[0]
	key := parts[1]

	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group : "+groupName, http.StatusNotFound)
		return
	}

	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 将 value 写到 response body 中，作为一个 a proto message.
	//ServeHTTP() 中使用 proto.Marshal() 编码 HTTP 响应
	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(body)
}

//Set : 实例化了一致性哈希算法，并且添加了传入的节点
func (p *HTTPPool) Set(nodes ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.nodes = consistentHash.New(defaultReplicas, nil)
	p.nodes.Add(nodes...)

	//为每一个节点创建了一个 HTTP 客户端 httpGetter
	p.httpGetters = make(map[string]*httpGetter, len(nodes))
	for _, node := range nodes {
		p.httpGetters[node] = &httpGetter{baseURL: node + p.basePath}
	}
}

//PickNode :包装了一致性哈希算法的 Get() 方法，根据具体的 key，选择节点，返回节点对应的 HTTP 客户端
func (p *HTTPPool) PickNode(key string) (node NodeGetter, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if node := p.nodes.Get(key); node != "" && node != p.self {
		p.Log("Pick node : %s", node)
		return p.httpGetters[node], true
	}
	return nil, false
}

var _ NodeGetter = (*httpGetter)(nil)
