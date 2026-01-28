run: build
	killall auth search load || true
	sh -c '(trap "kill 0" EXIT; ./build/auth & ./build/search & ./build/load)'

rundb: build
	mkdir -p build/data || true
	killall db || true
	sh -c '(trap "kill 0" EXIT; \
		cluster=":8200,:8201,:8202"; \
		./build/db -dir build/data -listen :8100 -node 0 -cluster $$cluster & \
		./build/db -dir build/data -listen :8101 -node 1 -cluster $$cluster & \
		./build/db -dir build/data -listen :8102 -node 2 -cluster $$cluster)'

build:
	cp -r libraries/static/ apps/auth/static/lib/
	cp -r libraries/static/ apps/search/static/lib/
	mkdir -p build
	go build -o build/auth apps/auth/main.go
	go build -o build/search apps/search/main.go
	go build -o build/load tools/load/main.go
	go build -o build/migrate tools/migrate/main.go
	go build -o build/dbcli tools/dbcli/main.go
	go build -o build/db tools/db/*.go

migrate: build
	./build/migrate

testdb:
	curl http://localhost:8100/?sql=$(shell echo "create table users (id, name)" | jq -Rr @uri)
	curl http://localhost:8100/?sql=$(shell echo "insert into users values (1, 'joe')" | jq -Rr @uri)
	curl http://localhost:8100/?sql=$(shell echo "select * from users" | jq -Rr @uri)

db:
	./build/dbcli

.PHONY: build
