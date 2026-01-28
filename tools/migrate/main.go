package main

import (
	"obwo/libraries/log"
	"obwo/libraries/util"
	"os"
	"slices"
	"strings"
)

type Migration struct {
	ID   string
	Up   string
	Down string
}

var migrations = []Migration{}

func M(id, up, down string) bool {
	migrations = append(migrations, Migration{id, up, down})
	return true
}

func main() {
	defer func() {
		if err := recover(); err != nil {
			log.Log("ERROR %v", err)
		}
	}()
	log.Setup("migrate")
	r := util.SQL(`select name from sqlite_master where type = 'table' and name = 'migrations'`)
	if len(r) == 0 {
		util.SQL(`create table migrations (id text not null primary key)`)
	}
	dbMigrations := util.SQL(`select id from migrations`)
	if len(os.Args) > 1 && os.Args[1] == "down" {
		slices.Reverse(migrations)
		for _, m := range migrations {
			for _, dm := range dbMigrations {
				if dm["id"].(string) == m.ID {
					log.Log("running down on %s", m.ID)
					util.SQL(strings.TrimSpace(m.Down))
					util.SQL(`delete from migrations where id = ?`, m.ID)
					return
				}
			}
		}
		log.Log("no migration to run down")
		return
	}
top:
	for _, m := range migrations {
		for _, dm := range dbMigrations {
			if dm["id"].(string) == m.ID {
				continue top
			}
		}
		log.Log("running %s", m.ID)
		util.SQL(strings.TrimSpace(m.Up))
		util.SQL(`insert into migrations values (?)`, m.ID)
	}
	log.Log("migrations: %d", len(migrations))
}

var _ = M(`auth_initial`, `
	create table users (
		id integer not null primary key,
		username text not null,
		password text not null,
		created integer not null,
		updated integer not null
	);
`, `
	drop table users;
`)
