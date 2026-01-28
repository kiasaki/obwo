package main

import (
	"embed"
	"regexp"
	"strconv"
	"time"

	"obwo/libraries/app"
	"obwo/libraries/util"
)

//go:embed static
var staticFS embed.FS

//go:embed templates
var templatesFS embed.FS

func main() {
	server := app.NewServer("auth")
	server.Port = 9001
	server.Templates(templatesFS)
	server.Static(staticFS)
	server.Handle("/settings/", handleSettings)
	server.Handle("/register/", handleRegister)
	server.Handle("/{$}", handleLogin)
	server.Handle("/", handleNotFound)
	server.Start()
}

func handleSettings(c *app.Context) {
	c.Render(200, "settings", util.J{})
}

func handleLogin(c *app.Context) {
	u := c.CurrentUser()
	if u != nil {
		c.Redirect(c.App(util.Or(c.Params["return"], "search")) + "/?auth=" + c.GetCookie(app.CookieName))
		return
	}

	if c.Req.Method == "POST" {
		users := c.SQL("select id, password from users where username = ?", c.Params["username"])
		if len(users) == 0 {
			c.Errors = append(c.Errors, "No user found with that username")
			goto render
		}
		user := users[0]
		if !util.CheckPassword(user.Get("password"), c.Params["password"]) {
			c.Errors = append(c.Errors, "Wrong password")
			goto render
		}
		token := util.CreateToken(user.Get("id"), util.Env("SECRET", "secret"), 7*24*60)
		c.SetCookie(app.CookieName, token)
		c.Redirect(c.App(util.Or(c.Params["return"], "search")) + "/?auth=" + token)
		return
	}
render:
	c.Render(200, "login", util.J{"title": "Login"})
}

func handleRegister(c *app.Context) {
	if c.Req.Method == "POST" {
		c.Errors = util.Validate(c.Params,
			util.ValidateRegexp("username", regexp.MustCompile("^[a-z0-9]{3,16}$")),
			util.ValidateUnique("username", "users", "username", ""),
			util.ValidateLength("password", 8, 64),
		)
		if c.Params["password"] != c.Params["passwordrepeat"] {
			c.Errors = append(c.Errors, "Password confirmation does not match")
		}
		if len(c.Errors) > 0 {
			goto render
		}
		id := util.NewId()
		now := time.Now().Unix()
		hashedPassword := util.CreatePassword(c.Params["password"])
		c.SQL("insert into users (id, username, password, created, updated) values (?, ?, ?, ?, ?)", id, c.Params["username"], hashedPassword, now, now)
		token := util.CreateToken(strconv.FormatInt(id, 10), util.Env("SECRET", "secret"), 7*24*60)
		c.SetCookie(app.CookieName, token)
		c.Redirect(c.App("search") + "/?auth=" + token)
		return
	}
render:
	c.Render(200, "register", util.J{"title": "Register"})
}

func handleNotFound(c *app.Context) {
	c.Text(404, "not found")
}
