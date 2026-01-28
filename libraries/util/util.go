package util

import (
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

type J map[string]interface{}

func (j J) Get(key string) string {
	if v, ok := j[key].(string); ok {
		return v
	}
	if v, ok := j[key].(float64); ok {
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
	return ""
}

func Check(err error) {
	if err != nil {
		panic(err)
	}
}

func Env(key, alt string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return alt
}

func Or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

var idSeq int64 = 0

func NewId() int64 {
	atomic.AddInt64(&idSeq, 1)
	id := (time.Now().UnixNano() / int64(time.Millisecond)) - 1262304000000
	id = id << 12
	id = id | (idSeq % 4096)
	return id
}
