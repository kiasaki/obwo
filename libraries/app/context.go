package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"obwo/libraries/util"
	"strings"
)

var CookieName string = "obwo"

type Context struct {
	Server      *Server
	Req         *http.Request
	Res         http.ResponseWriter
	Template    *template.Template
	Params      map[string]string
	Errors      []string
	currentUser util.J
}

func NewContext(s *Server, w http.ResponseWriter, r *http.Request) *Context {
	c := &Context{Server: s, Res: w, Req: r, Params: map[string]string{}}
	t, err := s.Template.Clone()
	util.Check(err)
	c.Template = t.Funcs(c.Funcs())

	util.Check(c.Req.ParseForm())
	for k, _ := range c.Req.URL.Query() {
		c.Params[k] = c.Req.URL.Query().Get(k)
	}
	for k, _ := range c.Req.Form {
		c.Params[k] = c.Req.Form.Get(k)
	}
	return c
}

func (c *Context) GetCookie(name string) string {
	if cookie, err := c.Req.Cookie(name); err == nil {
		return cookie.Value
	}
	return ""
}

func (c *Context) SetCookie(name, value string) {
	http.SetCookie(c.Res, &http.Cookie{
		Name:     name,
		Value:    value,
		HttpOnly: true,
		Path:     "/",
		MaxAge:   2147483647,
	})
}

func (c *Context) Header(key, value string) {
	c.Res.Header().Set(key, value)
}

func (c *Context) Redirect(url string, args ...interface{}) {
	if len(args) > 0 {
		url = fmt.Sprintf(url, args...)
	}
	c.Res.Header().Set("Location", url)
	c.Res.WriteHeader(302)
	c.Res.Write([]byte("Redirecting..."))
}

func (c *Context) Text(code int, s string) {
	c.Res.WriteHeader(code)
	c.Res.Write([]byte(s))
}

func (c *Context) Render(code int, name string, values util.J) {
	b := bytes.NewBuffer(nil)
	util.Check(c.Template.ExecuteTemplate(b, name+".html", values))
	c.Header("Content-Type", "text/html")
	c.Res.WriteHeader(code)
	c.Res.Write(b.Bytes())
}

func (c *Context) Funcs() template.FuncMap {
	return template.FuncMap{
		"title": strings.Title,
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
		"app": func(name string) template.URL {
			return template.URL(c.App(name))
		},
		"param": func(name string) string {
			return c.Params[name]
		},
		"errors": func() []string {
			return c.Errors
		},
		"currentUser": func() util.J {
			return c.CurrentUser()
		},
		"hasPrefix": func(s string, prefix string) bool {
			return strings.HasPrefix(s, prefix)
		},
		"formatSize": func(b int64) string {
			const unit = 1024
			if b < unit {
				return fmt.Sprintf("%d B", b)
			}
			div, exp := int64(unit), 0
			for n := b / unit; n >= unit; n /= unit {
				div *= unit
				exp++
			}
			return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
		},
		"json": func(i interface{}) string {
			bs, err := json.MarshalIndent(i, "", "  ")
			util.Check(err)
			return string(bs)
		},
		"mod": func(i, m int) int {
			return i % m
		},
		"div": func(a, b int64) float64 {
			return float64(a) / float64(b)
		},
		"add": func(a, b int) int {
			return a + b
		},
		"minus": func(a, b int) int {
			return a - b
		},
	}
}

func (c *Context) SQL(sql string, args ...interface{}) []util.J {
	return util.SQL(sql, args...)
}

func (c *Context) App(name string) string {
	scheme := "https://"
	if strings.Contains(c.Req.Host, "localhost") {
		scheme = "http://"
	}
	parts := strings.SplitN(c.Req.Host, ".", 2)
	return scheme + name + "." + parts[1]
}

func (c *Context) CurrentUser() util.J {
	if c.currentUser != nil {
		return c.currentUser
	}
	token := c.GetCookie(CookieName)
	if token == "" {
		return nil
	}
	id, ok := util.ValidateToken(token, util.Env("SECRET", "secret"))
	if !ok {
		return nil
	}
	users := c.SQL("select * from users where id = ?", id)
	if len(users) == 0 {
		return nil
	}
	c.currentUser = users[0]
	return c.currentUser
}
