package geecache

import (
	"GeeCache/consistenthash"
	pb "GeeCache/geecachepb"
	"fmt"
	"github.com/golang/protobuf/proto"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

/*
	分布式韩村需要实现节点间通信，建立基于 HTTP的通信机制进行节点间的通信
	如果一个节点启动了HTTP服务，那么这个节点就可以被其他节点访问。
*/

const (
	defaultBasePath  = "/_geecache/"
	defaultReplicase = 50
)

type (
	//HTTPPool 是HTTP服务器端
	HTTPPool struct {
		//记录自己的地址，包括主机名/IP 和端口号
		self string
		// 作为节点通讯地址的前缀 默认是 /_geecache/
		basePath string
		mu       sync.Mutex
		//类型是一致性哈希算法的Map，用来根据具体的key选择节点
		peers *consistenthash.Map
		//映射远程节点与对应的 httpGetter 每一个远程节点对应一个 httpGetter
		//因为 httpGetter 与远程节点的地址 baseURL 有关
		httpGetters map[string]*httpGetter
	}

	//httpGetter 具体的 HTTP 客户端 实现 PeerGetter 接口
	httpGetter struct {
		//baseURL 表示将要访问的远程节点的地址 eg:http://example.com/_geecache/
		baseURL string
	}
)

// NewHTTPPool initializes an HTTP pool of peers.
func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		//   "http://IP:port
		self:     self,
		basePath: defaultBasePath,
	}
}

//Log 进行节点通信记录
func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	p.Log("%s %s", r.Method, r.URL.Path)
	// /<basepath>/<groupname>/<key>  required
	//将 SplitN 切成由 sep 分隔的子字符串，并返回这些分隔符之间的子字符串的一个片段。
	//计数决定了要返回的子字符串的数量
	// 例如 /_geecache/name/yxm  -->  得到的是:/name/yxm  ?? 看看切分的结果
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	//规定的是获取数据的固定为两部分 切分后不为两部分，则请求的Path 不正确
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// 获取组名 和 key
	groupName := parts[0]
	key := parts[1]

	//获取命名空间的一个组
	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}
	// 通过key 获取对应的View view 是为了遍历传输
	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write the value to the response body as a proto message.
	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//只能传输二进制
	w.Header().Set("Content-Type", "application/octet-stream")
	_, err = w.Write(body)
	if err == nil {
		log.Fatal("write failed", err)
	}

}

//Set 实例化了一致性哈希算法，并且添加了传入的节点
//并为每一个节点创建了一个 HTTP 客户端：httpGetter
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	//实例话一致性哈希算法
	p.peers = consistenthash.New(defaultReplicase, nil)
	//添加节点
	p.peers.Add(peers...)
	// 初始化 httpGetter
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers {
		//对应每一个 httpGetter 添加相对应的 URL地址
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}
	}
}

//PickPeer 包装了一致性哈希算法的 Get 方法，根据具体的 key 选择节点，返回节点对应的 HTTP 客户端
func (p *HTTPPool) PickPeer(key string) (peer PeerGetter, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// peer == "" 是 哈希环上无节点的情况 peer == p.self 是请求本地节点的情况
	// p.peers.Get(key) 是 获取节点的真实名称
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("pick peer %s", peer)
		return p.httpGetters[peer], true
	}
	return nil, false
}

//Get 从对应 group 查找缓存值   获取返回值，并转换为 []bytes类型
//func (h *httpGetter) Get(group string, key string) ([]byte, error) {
//	//根据格式说明符设置 Sprintf 格式，并返回结果字符串。
//	u := fmt.Sprintf(
//		"%v%v/%v",
//		h.baseURL,
//		// QueryEscape 对字符串进行转义，以便可以安全地将其放置在 URL 查询中
//		url.QueryEscape(group),
//		url.QueryEscape(key),
//	)
//
//	// http.Get() 方法获取返回值，并转换为 []byte类型
//	res, err := http.Get(u)
//	if err != nil {
//		return nil, err
//	}
//	defer res.Body.Close()
//	//如果相应结果不是成功
//	if res.StatusCode != http.StatusOK {
//		return nil, fmt.Errorf("sever return: %v", res.Status)
//	}
//
//	bytes, err := ioutil.ReadAll(res.Body)
//	if err != nil {
//		return nil, fmt.Errorf("reading response body: %v", err)
//	}
//	return bytes, nil
//}

// Get 中 使用 proto.UnMarshall()解码 HTTP响应
func (h *httpGetter) Get(in *pb.Request, out *pb.Response) error {
	u := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		// 获取 request字段中的组
		url.QueryEscape(in.GetGroup()),
		// 获取 request字段中的key
		url.QueryEscape(in.GetKey()),
	)
	res, err := http.Get(u)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %v", res.Status)
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}

	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}

	return nil
}
