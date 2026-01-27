package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"obwo/libraries/util"

	_ "modernc.org/sqlite"
)

type Command struct {
	Query string   `json:"q"`
	Args  []string `json:"a"`
}

type statemachine struct {
	db *sql.DB
}

func main() {
	log.Default().SetOutput(io.Discard)
	dataDir := flag.String("dir", "", "directory to store data in")
	listen := flag.String("listen", ":8100", "http server port")
	index := flag.Int("node", 0, "node index")
	cluster := flag.String("cluster", ":8200", "comma separated addresses")
	flag.Parse()
	members := []ClusterMember{}
	for i, m := range strings.Split(*cluster, ",") {
		members = append(members, ClusterMember{Id: uint64(i), Address: m})
	}

	db, err := sql.Open("sqlite", path.Join(*dataDir, fmt.Sprintf("db%d.sqlite3", *index))+"?_pragma=journal_mode=WAL&_time_integer_format=unix_milli")
	check(err)
	sm := &statemachine{db: db}
	s := NewServer(members, sm, *dataDir, *index)
	s.Start()

	check(http.ListenAndServe(*listen, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				sendJson(w, 500, util.J{"error": fmt.Sprintf("%v", err)})
			}
		}()
		sql := r.URL.Query().Get("sql")
		if sql == "" {
			panic("missing sql arg")
		}
		args := r.URL.Query()["args"]
		c := Command{Query: sql, Args: args}
		var err error
		var result []byte
		if strings.HasPrefix(strings.ToLower(sql), "select") {
			result, err = sm.Execute(c)
		} else {
			bs, err := json.Marshal(c)
			check(err)
			result, err = s.Apply(bs)
		}
		check(err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(result)
	})))
}

func sendJson(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	check(json.NewEncoder(w).Encode(v))
}

func (s *statemachine) Apply(cmd []byte) ([]byte, error) {
	c := Command{}
	check(json.Unmarshal(cmd, &c))
	return s.Execute(c)
}

func (s *statemachine) Execute(c Command) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	args := []interface{}{}
	for _, v := range c.Args {
		args = append(args, v)
	}
	rows, err := s.db.QueryContext(ctx, c.Query, args...)
	if err != nil {
		return nil, err
	}

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	results := []util.J{}
	for rows.Next() {
		result := util.J{}
		pointers := make([]interface{}, len(cols))
		row := make([]interface{}, len(cols))
		for i, _ := range row {
			pointers[i] = &row[i]
		}
		err := rows.Scan(pointers...)
		if err != nil {
			return nil, err
		}
		for i, col := range cols {
			result[col] = row[i]
		}
		results = append(results, result)
	}
	return json.Marshal(results)
}
