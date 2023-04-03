SOURCE_FILES := $(shell find . -iname '*.go' ! -ipath '*/vendor/*')
GIT_TAG := $(shell git describe --tags)
LDFLAGS := -ldflags "-X main.Version=$(GIT_TAG)"

sensor-probe: $(SOURCE_FILES)
	go build $(LDFLAGS)

.PHONY: build-arm
build-arm: $(SOURCE_FILES)
	env GOOS=linux GOARCH=arm GOARM=5 go build $(LDFLAGS) -o sensor-probe.arm5

.PHONY: test
test:
	go test -v ./...

.PHONY: lint
lint:
	go vet ./...
	staticcheck ./...
