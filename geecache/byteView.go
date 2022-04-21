package geecache

// ByteView 只有一个数据成员，bv []byte，bv 将会存储真实的缓存值。选择 byte 类型是为了能够支持任意的数据类型的存储，例如字符串、图片等
type ByteView struct {
	bv []byte
}

// Len 方法，在 lru.Cache 的实现中，要求被缓存对象必须实现 Value 接口，即 Len() int 方法，返回其所占的内存大小
func (b ByteView) Len() int {
	return len(b.bv)
}

// ByteSlice 方法: bv 是只读的，使用 ByteSlice() 方法返回一个拷贝，防止缓存值被外部程序修改
func (b ByteView) ByteSlice() []byte {
	return cloneBytes(b.bv)
}

// String 方法: returns the data as a string, making a copy if necessary.
func (b ByteView) String() string {
	return string(b.bv)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
