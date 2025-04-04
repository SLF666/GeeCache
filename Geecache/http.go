package geecache

import (
	"fmt"
	"geecache/consistenthash"
	pb "geecache/proto"
	"google.golang.org/protobuf/proto"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

//提供被其他节点访问的HTTP接口

const (
	defaultBasePath = "/_geecache/"
	defaultReplicas = 50
)

// 承载节点间HTTP通信的核心结构
// 实现了PeerPicker节点选择器
type HTTPPool struct {
	self     string //记录自己的地址
	basePath string //节点间通信的地址前缀

	mu          sync.Mutex
	peers       *consistenthash.Map    //管理节点
	httpGetters map[string]*httpGetter //记录每个节点名称对应的httpGetter，也就是客户端
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

// 实例化一致性哈希，并添加节点
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.peers = consistenthash.New(defaultReplicas, nil)
	p.peers.Add(peers...)
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers {
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}
	}
}

// 根据key返回节点的客户端
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	//这里Get返回的是真实节点的名称
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("Pick peer %s", peer)
		return p.httpGetters[peer], true
	}
	return nil, false
}

var _ PeerPicker = (*HTTPPool)(nil)

// 服务端实现，实现了http.Handler接口
//
//	从组返回缓存，并下载
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//判断是否以/geecache/开头
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	p.Log("%s %s", r.Method, r.URL.Path)
	// /<basepath>/<groupname>/<key> required
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	//提取组名和key
	groupName := parts[0]
	key := parts[1]

	//判断组是否存在
	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, fmt.Sprintf("group %s nod found", groupName), http.StatusNotFound)
		return
	}

	//获取缓存值
	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//通常用于下载文件，尤其是当服务器无法确定文件的具体类型时，
	//或者当客户端需要将响应内容作为文件下载时。
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(body) //返回缓存值
}

// 定义 Gin 版本的处理函数
func (p *HTTPPool) GinHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 路径格式校验（由 Gin 路由确保，无需手动检查前缀）
		// 例如路由定义为 "/_geecache/:group/:key"

		// 2. 提取 group 和 key
		groupName := c.Param("group")
		key := c.Param("key")

		// 3. 判断组是否存在
		group := GetGroup(groupName)
		if group == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("group %s not found", groupName)})
			return
		}

		// 4. 获取缓存值
		view, err := group.Get(key)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 5. 返回二进制数据
		c.Data(http.StatusOK, "application/octet-stream", body)
	}
}

// 客户端，实现了PeerGetter接口
// 能够通过http从远程节点获取缓存
type httpGetter struct {
	baseURL string //将要访问的远程节点地址
}

// 通过http请求从远程服务器获取数据
func (h *httpGetter) Get(in *pb.Request, out *pb.Response) error {
	u := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		url.QueryEscape(in.GetGroup()), //确保特殊字符被正确编码，以避免 URL 格式错误
		url.QueryEscape(in.GetKey()),
	)
	//发送get请求
	res, err := http.Get(u)
	if err != nil {
		return err
	}
	defer res.Body.Close() //养成好习惯，及时关闭连接

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %v", res.Status)
	}

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}
	if err := proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("unmarshaling response body: %v", err)
	}
	return nil
}

// 显式检查，它是在编译时检查 httpGetter 类型是否实现了 PeerGetter 接口。
var _ PeerGetter = (*httpGetter)(nil)
