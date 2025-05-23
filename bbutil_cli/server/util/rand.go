package util

import (
	"crypto/rand"
	"encoding/binary"
)

func GetRandInt64() int64 {
	var randomNumber int64

	for {
		// 生成8字节的随机数据
		randomBytes := make([]byte, 8)
		rand.Read(randomBytes)
		// 将随机数据转换为int64
		randomNumber = int64(binary.BigEndian.Uint64(randomBytes))
		if randomNumber > 0 {
			break
		}
	}
	return randomNumber
}
