package util

import (
	"bbutil_cli/common"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

func MD5(str string) string {
	data := []byte(str)
	return fmt.Sprintf("%x", md5.Sum(data))
}

var aesKey = []byte("laksjebc9nkl875s")

func AESEncoding(src string) (string, error) {
	if src == "" {
		return "", nil
	}
	srcByte := []byte(src)
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		common.Logger.Error(err)
		return src, err
	}
	//GCM加密模式
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		common.Logger.Error(err)
		return src, err
	}
	// 生成随机nonce
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		common.Logger.Error(err)
		return src, err
	}
	//加密
	cipherText := aesGCM.Seal(nil, nonce, srcByte, nil)
	return base64.StdEncoding.EncodeToString(cipherText) + "@" + base64.StdEncoding.EncodeToString(nonce), nil
}

func AESDecoding(pwd string) (string, error) {
	if pwd == "" {
		return "", nil
	}

	strArr := strings.Split(pwd, "@")
	decryByte, err := base64.StdEncoding.DecodeString(strArr[0])
	if err != nil {
		common.Logger.Error(err)
		return "", err
	}
	nonceByte, err := base64.StdEncoding.DecodeString(strArr[1])
	if err != nil {
		common.Logger.Error(err)
		return "", err
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		common.Logger.Error(err)
		return pwd, err
	}
	//GCM加密模式
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		common.Logger.Error(err)
		return pwd, err
	}
	// 解密
	decryptedText, err := aesGCM.Open(nil, nonceByte, decryByte, nil)
	if err != nil {
		common.Logger.Error(err)
		return pwd, err
	}
	return string(decryptedText), nil

}
