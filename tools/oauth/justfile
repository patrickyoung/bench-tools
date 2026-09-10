default:
    go test ./...

race:
    go test -race ./...

build:
    mkdir -p bin
    go build -trimpath -o bin/oauth ./cmd/oauth

install:
    ./install.sh
