.PHONY: build run clean test lint tidy

BINARY := paravizor
CMD    := ./cmd/paravizor

build:
	go build -buildvcs=false -o $(BINARY) $(CMD)

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)

test:
	go test ./...

lint:
	golangci-lint run ./...

tidy:
	go mod tidy
