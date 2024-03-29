package util

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/gob"
	"fmt"
	"hash/crc32"
	"hash/crc64"
	"io"
	"math/rand"
	"os"
	"strings"
	"time"
)

const (
	IndentSize    = 4
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

var (
	crc64_table = crc64.MakeTable(crc64.ECMA)
	crc32_table = crc32.MakeTable(0xD5828281)
	_randSrc    = rand.NewSource(time.Now().UnixNano())
)

func RandString(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, _randSrc.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = _randSrc.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func RandInt64() int64 {
	return _randSrc.Int63()
}

func RandInt() int {
	return int(_randSrc.Int63())
}

func Md5sum(raw []byte) string {
	h := md5.New()
	h.Write(raw)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func Sha1sum(raw []byte) string {
	return fmt.Sprintf("%x", sha1.Sum(raw))
}

func FileMd5sum(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()

	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func Sum64(raw []byte) uint64 {
	h := crc64.New(crc64_table)
	h.Write(raw)
	//io.WriteString(h, raw)
	return h.Sum64()
}

func Sum32(raw []byte) uint32 {
	return crc32.Checksum(raw, crc32_table)
}

func StructMd5(in interface{}) (string, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(in)
	if err != nil {
		return "", err
	}
	return Md5sum(buf.Bytes()), nil
}

func HashPath(filePath string, level int) string {
	filePath = strings.Trim(filePath, "/")
	m := md5.Sum([]byte(filePath))

	if level < 0 {
		level = 0
	}
	if level > md5.Size {
		level = md5.Size
	}

	var buf bytes.Buffer
	for i := 0; i < level; i++ {
		fmt.Fprintf(&buf, "%02x/", m[i])
	}
	fmt.Fprintf(&buf, filePath)

	return buf.String()
}
