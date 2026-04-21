BINARY := airpath
GOCACHE ?= /tmp/go-cache

.PHONY: build test vet fmt check clean example

build:
	GOCACHE=$(GOCACHE) go build -o $(BINARY) ./cmd/airpath/

test:
	GOCACHE=$(GOCACHE) go test ./...

test-v:
	GOCACHE=$(GOCACHE) go test -v ./...

vet:
	GOCACHE=$(GOCACHE) go vet ./...

fmt:
	gofmt -w .

check: vet test

example: build
	./$(BINARY) generate -scene examples/small_room.json -output ./output/

clean:
	rm -f $(BINARY)
	rm -rf output/
