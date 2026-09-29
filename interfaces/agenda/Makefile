.PHONY: build check clean

build:
	go build -o bin/agenda-ui .

check:
	go test ./...
	go test -race ./...
	go vet ./...
	go build -o bin/agenda-ui .

clean:
	rm -rf bin
