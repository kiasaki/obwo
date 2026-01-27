package app

import (
	"embed"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"obwo/libraries/log"
	"obwo/libraries/util"
	"runtime/debug"
	"time"
)

type Server struct {
	ServiceName string
	Port        int
	Mux         *http.ServeMux
	Template    *template.Template
}

func NewServer(serviceName string) *Server {
	log.Setup(serviceName)
	s := &Server{}
	s.ServiceName = serviceName
	port := util.Env("PORT", util.IntToString(9000+rand.Intn(100)))
	s.Port = util.StringToInt(port)
	s.Mux = http.NewServeMux()
	return s
}

func (s *Server) Templates(templatesFS embed.FS) {
	t := template.New("global")
	t.Funcs((&Context{}).Funcs())
	var err error
	s.Template, err = t.ParseFS(templatesFS, "templates/*.html")
	util.Check(err)
}

func (s *Server) Static(staticFS embed.FS) {
	s.Mux.Handle("/static/", http.FileServer(http.FS(staticFS)))
}

func (s *Server) Handle(pattern string, handler func(c *Context)) {
	s.Mux.HandleFunc(pattern, s.wrap(handler))
}

func (s *Server) wrap(handler func(c *Context)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		c := NewContext(s, w, r)
		start := time.Now()
		defer func() {
			log.Log("%s %s %v", r.Method, r.URL.Path, time.Since(start))
			if err := recover(); err != nil {
				log.Log("ERROR %v", err)
				log.Log("%s", string(debug.Stack()))
				c.Text(500, fmt.Sprintf("ERROR %v", err))
			}
		}()
		handler(c)
	}
}

func (s *Server) Start() {
	log.Log("listening on %d", s.Port)
	http.ListenAndServe("0.0.0.0:"+util.IntToString(s.Port), s.Mux)
}
