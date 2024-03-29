SOURCE_FILES := $(shell find . -iname '*.go' ! -ipath '*/vendor/*')
GIT_TAG := $(shell git describe --tags)
LDFLAGS := -ldflags "-X main.Version=$(GIT_TAG)"

sensor-probe: $(SOURCE_FILES)
	env CGO_ENABLED=0 go build $(LDFLAGS)

.PHONY: build-arm
build-arm: $(SOURCE_FILES)
	env CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=5 go build $(LDFLAGS) -o sensor-probe.arm5

.PHONY: test
test:
	go test -v ./...

.PHONY: lint
lint:
	go vet ./...
	staticcheck ./...

.PHONY: clean
clean:
	rm -f sensor-probe sensor-probe.arm5
