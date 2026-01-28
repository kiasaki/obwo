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

type Context struct {
	Server   *Server
	Req      *http.Request
	Res      http.ResponseWriter
	Template *template.Template
}

func NewContext(s *Server, w http.ResponseWriter, r *http.Request) *Context {
	c := &Context{Server: s, Res: w, Req: r}
	t, err := s.Template.Clone()
	util.Check(err)
	c.Template = t.Funcs(c.Funcs())
	return c
}

func (c *Context) Header(key, value string) {
	c.Res.Header().Set(key, value)
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
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
		"app": func(name string) template.URL {
			scheme := "https://"
			if strings.Contains(c.Req.Host, "localhost") {
				scheme = "http://"
			}
			parts := strings.SplitN(c.Req.Host, ".", 2)
			return template.URL(scheme + name + "." + parts[1])
		},
		"currentUser": func() util.J {
			return nil
		},
		"title": strings.Title,
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

func (c *Context) Query(sql string, args ...interface{}) []util.J {
	return util.SQL(sql, args...)
}
