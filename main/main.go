package main

import (
	geecache "GeeCache"
	"flag"
	"fmt"
	"log"
	"net/http"
)

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func createGroup() *geecache.Group {
	return geecache.NewGroup("scores", 2<<10, geecache.GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))
}

//startCacheServer 用来启动缓存服务器 创建 HTTPPool 添加节点信息 注册到 gee中 启动 HTTP服务 (共3个端口， 8001/8002/8003) 用户不感知
func startCacheServer(addr string, addrs []string, gee *geecache.Group) {
	//peers 是一个 HTTPPool 初始化节点
	peers := geecache.NewHTTPPool(addr)
	//添加节点名
	peers.Set(addrs...)
	//将peers 注入到 group 中
	gee.RegisterPeers(peers)
	log.Println("geecache is running at", addr)
	log.Fatal(http.ListenAndServe(addr[7:], peers))
}

//startAPIServer 用来启动一个API服务(端口 9999)，与用户进行交互，用户感知
func startAPIServer(apiAddr string, gee *geecache.Group) {
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// 获取 key对应的 value
			key := r.URL.Query().Get("key")
			// 获取缓存值
			view, err := gee.Get(key)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, err = w.Write(view.ByteSlice())
			if err == nil {
				log.Println("Write failed", err)
			}

		}))
	log.Println("fontend server is running at", apiAddr)
	log.Fatal(http.ListenAndServe(apiAddr[7:], nil))

}

//main 函数需要命令行传入 port 和 api 2个参数 用来指定启动端口 HTTP服务
func main() {
	var port int
	var api bool
	flag.IntVar(&port, "port", 8001, "Geecache server port")
	flag.BoolVar(&api, "api", false, "Start a api server?")
	flag.Parse()

	apiAddr := "http://localhost:9999"
	addrMap := map[int]string{
		8001: "http://localhost:8001",
		8002: "http://localhost:8002",
		8003: "http://localhost:8003",
	}

	var addrs []string
	for _, v := range addrMap {
		addrs = append(addrs, v)
	}

	gee := createGroup()
	if api {
		go startAPIServer(apiAddr, gee)
	}
	startCacheServer(addrMap[port], []string(addrs), gee)
}
