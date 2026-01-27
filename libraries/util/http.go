package util

import (
	"encoding/json"
	"fmt"
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
	if resp.StatusCode != 200 {
		panic(fmt.Errorf("HttpGet: status not ok (%d)", resp.StatusCode))
	}
	Check(json.NewDecoder(resp.Body).Decode(v))
}
