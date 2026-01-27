run: build
	killall auth search load || true
	sh -c '(trap "kill 0" EXIT; ./build/auth & ./build/search & ./build/load)'

build:
	cp -r libraries/static/ apps/auth/static/lib/
	cp -r libraries/static/ apps/search/static/lib/
	mkdir -p build
	go build -o build/auth apps/auth/main.go
	go build -o build/search apps/search/main.go
	go build -o build/load tools/load/main.go

.PHONY: build
