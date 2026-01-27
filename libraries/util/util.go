package util

import "os"

type J map[string]interface{}

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
