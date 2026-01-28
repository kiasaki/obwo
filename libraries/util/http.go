package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func HttpGet(v interface{}, url string, headers map[string]string) {
	req, err := http.NewRequest("GET", url, nil)
	Check(err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	Check(err)
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	Check(err)
	if resp.StatusCode != 200 {
		panic(fmt.Errorf("HttpGet: status not ok %d: %s", resp.StatusCode, string(bs)))
	}
	Check(json.Unmarshal(bs, v))
}
