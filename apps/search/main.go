package main

import (
	"embed"
	"net/url"
	"obwo/libraries/app"
	"obwo/libraries/util"
)

//go:embed static
var staticFS embed.FS

//go:embed templates
var templatesFS embed.FS

func main() {
	server := app.NewServer("search")
	server.Port = 9002
	server.Templates(templatesFS)
	server.Static(staticFS)
	server.Handle("/{$}", handleSearch)
	server.Handle("/", handleNotFound)
	server.Start()
}

func handleSearch(c *app.Context) {
	q := c.Req.URL.Query().Get("q")
	results := []interface{}{}
	if q != "" {
		results = performSearch(q)
	}
	c.Render(200, "search", util.J{
		"title":   q,
		"q":       q,
		"results": results,
	})
}

func handleNotFound(c *app.Context) {
	c.Text(404, "not found")
}

func performSearch(query string) []interface{} {
	u := "https://api.search.brave.com/res/v1/web/search?q=" + url.QueryEscape(query)
	r := util.J{}
	util.HttpGet(&r, u, map[string]string{
		"Accept":               "application/json",
		"X-Subscription-Token": util.Env("BRAVE_API_KEY", ""),
	})
	web, ok := r["web"].(map[string]interface{})
	if !ok {
		panic("performSearch: !web")
	}
	res, ok := web["results"].([]interface{})
	if !ok {
		panic("performSearch: !results")
	}
	return res
}
