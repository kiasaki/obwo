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

type State struct {
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
	sm := &State{db: db}
	s := NewServer(members, sm, *dataDir, *index)
	s.Restore = sm.Restore
	s.Persist = sm.Persist
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
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(sql)), "select") {
			result, err = sm.Execute(c)
		} else {
			bs, e := json.Marshal(c)
			check(e)
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

func (s *State) Apply(cmd []byte) ([]byte, error) {
	if len(cmd) == 0 {
		return []byte("[]"), nil
	}
	c := Command{}
	check(json.Unmarshal(cmd, &c))
	return s.Execute(c)
}

func (s *State) Execute(c Command) ([]byte, error) {
	args := []interface{}{}
	for _, v := range c.Args {
		args = append(args, v)
	}
	results, err := execute(s.db, c.Query, args...)
	if err != nil {
		return nil, err
	}
	return json.Marshal(results)
}

func execute(db *sql.DB, sql string, args ...any) ([]util.J, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	rows, err := db.QueryContext(ctx, sql, args...)
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
	return results, nil
}

func (s *State) Restore() PersistentState {
	state := PersistentState{}
	_, err := execute(s.db, "create table if not exists journal (id integer not null primary key, voted integer not null, term integer not null, command text not null)")
	check(err)
	entries, err := execute(s.db, "select * from journal")
	check(err)
	if len(entries) == 0 {
		return state
	}
	for _, e := range entries {
		state.Log = append(state.Log, Entry{
			Id:      e["id"].(int64),
			Term:    uint64(e["term"].(int64)),
			Command: []byte(e["command"].(string)),
		})
	}
	state.VotedFor = uint64(entries[len(entries)-1]["voted"].(int64))
	state.CurrentTerm = uint64(entries[len(entries)-1]["term"].(int64))
	return state
}

func (s *State) Persist(state PersistentState) {
	r, err := execute(s.db, "select coalesce(max(id), -1) as id from journal")
	check(err)
	id := r[0]["id"].(int64)
	for _, l := range state.Log {
		if l.Id <= id {
			continue
		}
		_, err := execute(s.db, "insert into journal (id, voted, term, command) values (?, ?, ?, ?)", l.Id, state.VotedFor, l.Term, string(l.Command))
		check(err)
	}
}
