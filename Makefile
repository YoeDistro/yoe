.PHONY: build test clean

build:
	CGO_ENABLED=0 go build -o yoe ./cmd/yoe

test:
	go test ./...

clean:
	rm -f yoe
