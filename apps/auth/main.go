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
	server := app.NewServer("auth")
	server.Templates(templatesFS)
	server.Static(staticFS)
	server.Handle("/register/", handleRegister)
	server.Handle("/{$}", handleLogin)
	server.Handle("/", handleNotFound)
	server.Start()
}

func handleLogin(c *app.Context) {
	c.Render(200, "login", util.J{
		"title": "Login",
	})
}

func handleRegister(c *app.Context) {
	c.Render(200, "register", util.J{
		"title": "Register",
	})
}

func handleNotFound(c *app.Context) {
	c.Text(404, "not found")
}
