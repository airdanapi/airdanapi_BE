.PHONY: run test tidy

run:
	go run ./cmd/server

test:
	go test ./...

tidy:
	go mod tidy
