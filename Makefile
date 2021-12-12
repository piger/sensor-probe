SOURCE_FILES := $(shell find . -iname '*.go' ! -ipath '*/vendor/*')

sensor-probe: $(SOURCE_FILES)
	go build .

build-arm: $(SOURCE_FILES)
	env GOOS=linux GOARCH=arm GOARM=5 go build -o sensor-probe.arm5 .

test:
	go test -v ./...

lint:
	go vet ./...
	golangci-lint run

.PHONY: test lint build-arm
