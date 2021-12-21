package geecache

import pb "GeeCache/geecachepb"

type (
	PeerPicker interface {
		//PickPeer 根据传入的key选择相应节点PeerGetter
		PickPeer(key string) (peer PeerGetter, ok bool)
	}
	//PeerGetter 相当于 HTTP客户端
	PeerGetter interface {
		//Get 从对应 group 查找缓存值。
		Get(in *pb.Request, out *pb.Response) error
	}
)
