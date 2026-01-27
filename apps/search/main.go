package main

import (
	"embed"
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
	c.Render(200, "search", util.J{
		"title": q,
		"q":     q,
	})
}

func handleNotFound(c *app.Context) {
	c.Text(404, "not found")
}
