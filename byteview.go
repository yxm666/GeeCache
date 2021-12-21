package geecache

type ByteView struct {
	// b将会存储真实的缓存值 byte支持任意的数据类型的存储
	b []byte
}

//Len 要求缓存对象必须实现 Value接口
func (v ByteView) Len() int {
	return len(v.b)
}
func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

//ByteSlice 返回一个拷贝，防止缓存值被外部程序修改
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

func (v ByteView) String() string {
	return string(v.b)
}
