package geecache

//节点选择器能返回客户端，客户端能从远端节点查找缓存

import pb "geecache/proto"

// 节点选择器
type PeerPicker interface {
	// 根据key返回节点客户端
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// 客户端需要实现的接口
type PeerGetter interface {
	// 从对应的group中查找缓存
	//Get(group string, key string) ([]byte, error)
	Get(in *pb.Request, out *pb.Response) error
}
