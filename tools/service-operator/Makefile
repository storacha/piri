.PHONY: build clean install test

build:
	go build -o service-operator

clean:
	rm -f service-operator

install: build
	go install

test:
	go test ./...