package util

import (
	"fmt"
	"net/url"
)

func SQL(sql string, args ...interface{}) []J {
	var v []J
	values := url.Values{}
	values.Set("sql", sql)
	for _, a := range args {
		values["args"] = append(values["args"], fmt.Sprintf("%v", a))
	}
	db := Env("DATABASE_URL", "http://db.localhost:8000")
	HttpGet(&v, db+"/?"+values.Encode(), nil)
	return v
}
