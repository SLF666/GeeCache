package geecache

//真实的缓存数据，选择byte是因为可以支持多种数据类型
//缓存值的抽象，将不同的数据类型抽象成统一的数据类型，方便缓存操作

type ByteView struct {
	b []byte
}

//实现Value接口
func (v ByteView) Len() int {
	return len(v.b)
}

//返回一个byte的切片，返回切片的拷贝，防止外部修改
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

//返回string
func (v ByteView) String() string {
	return string(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}