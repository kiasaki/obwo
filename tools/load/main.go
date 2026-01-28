package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"obwo/libraries/log"
	"strings"
	"sync"
)

func main() {
	log.Setup("load")

	dbURL0, _ := url.Parse("http://localhost:8100")
	dbURL1, _ := url.Parse("http://localhost:8101")
	dbURL2, _ := url.Parse("http://localhost:8102")
	dbProxies := []*httputil.ReverseProxy{
		httputil.NewSingleHostReverseProxy(dbURL0),
		httputil.NewSingleHostReverseProxy(dbURL1),
		httputil.NewSingleHostReverseProxy(dbURL2),
	}
	var dbCounter int
	var dbMutex sync.Mutex

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
		case "db":
			dbMutex.Lock()
			index := dbCounter % 3
			dbCounter++
			dbMutex.Unlock()
			dbProxies[index].ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
}
