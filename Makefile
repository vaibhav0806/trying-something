.PHONY: run test deps clean

run:
	go run .

test:
	go test -v ./...

deps:
	go mod tidy
	go mod download

clean:
	rm -f urls.db
