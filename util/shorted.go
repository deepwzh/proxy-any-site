package util

import "hash/fnv"

const (
	alphabet    = "abcdefghijklmnopqrstuvwxyz0123456789"
	base        = uint64(len(alphabet))
	shortURLLen = 8 // 缩短后的 URL 长度
)

// 计算 URL 的哈希值
func getHash(url string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(url))
	return h.Sum64()
}

// 根据原始 URL 生成缩短后的 URL
func ShortenURL(url string) string {
	hash := getHash(url)
	shortURL := ""
	for i := 0; i < shortURLLen; i++ {
		index := hash % base
		char := string(alphabet[index])
		shortURL += char
		hash = hash / base
	}
	return shortURL
}
