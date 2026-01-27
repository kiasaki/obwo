package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"obwo/libraries/log"
	"strings"
)

func main() {
	log.Setup("load")

	authURL, _ := url.Parse("http://localhost:9001")
	searchURL, _ := url.Parse("http://localhost:9002")
	authProxy := httputil.NewSingleHostReverseProxy(authURL)
	searchProxy := httputil.NewSingleHostReverseProxy(searchURL)

	log.Log("listening on 8000")
	http.ListenAndServe(":8000", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.Split(r.Host, ".")[0] {
		case "auth":
			authProxy.ServeHTTP(w, r)
		case "search":
			searchProxy.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
}
