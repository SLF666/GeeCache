package geecache

//与外部交互，更偏向应用层，节点实例是由HTTPPool处理，负责网络通信和节点选择。
//缓存组

import (
	"fmt"
	pb "geecache/proto"
	"geecache/singleflight"
	"log"
	"sync"
)

type Getter interface {
	//回调函数，缓存未命中时，获取源数据的方式
	Get(key string) ([]byte, error)
}

// 定义函数类型，并实现接口Getter
// 很实用，这样就可以把函数当作参数传入
type GetterFunc func(key string) ([]byte, error)

// 定义一个函数类型 F，并且实现接口 A 的方法，然后在这个方法中调用自己。
// 这是 Go 语言中将其他函数（参数返回值定义与 F 一致）转换为接口 A 的常用技巧。
// 实现接口
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// 一个单独的缓存组
type Group struct {
	name      string //名称
	getter    Getter //缓存未命中时获取数据的回调
	mainCache cache

	peers PeerPicker //节点选择器

	loader *singleflight.Group
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group) //储存所有的缓存组
)

// 实例化一个缓存组
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	//回调函数不能为空
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		loader:    &singleflight.Group{},
	}
	groups[name] = g
	return g
}

// 返回缓存组，没有则返回空
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	//先从本地节点缓存中查找
	if v, ok := g.mainCache.get(key); ok {
		//找到就返回
		log.Println("[GeeCache] 在本地找到")
		return v, nil
	}

	//缓存没有命中，调用下载函数
	return g.load(key)
}

// 下载数据
func (g *Group) load(key string) (ByteView, error) {
	viewi, err := g.loader.Do(key, func() (interface{}, error) {
		if g.peers != nil {
			//根据key选择节点
			if peer, ok := g.peers.PickPeer(key); ok {
				value, err := g.getFromPeer(peer, key)
				if err == nil {
					return value, nil
				}
				log.Println("[GeeCache] Failed to get from peer", err)
			}
		}
		//如果是自己负责的就调用本地下载
		return g.getLocally(key)
	})

	if err == nil {
		return viewi.(ByteView), nil
	}
	return ByteView{}, err
}

// 调用回调函数获取数据，并将数据保存到本地缓存中
func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)
	//如果获取数据失败，则返回错误
	if err != nil {
		return ByteView{}, err
	}
	value := ByteView{b: cloneBytes(bytes)}
	//将数据保存到本地节点缓存中
	g.populateCache(key, value)
	return value, nil
}

// 将数据保存到本地节点缓存中
func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}

// 注册节点选择器
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

// 从远程节点获取数据
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	//bytes, err := peer.Get(g.name, key)
	//if err != nil {
	//	return ByteView{}, err
	//}
	//return ByteView{b: bytes}, nil
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	err := peer.Get(req, res)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: cloneBytes(res.Value)}, nil
}
