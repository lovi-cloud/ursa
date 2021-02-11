.DEFAULT_GOAL := build

CURRENT_REVISION = $(shell git rev-parse --short HEAD)
BUILD_LDFLAGS = "-X main.revision=$(CURRENT_REVISION)"

cmd/ursa/static/ursa-bonder:
	mkdir -p cmd/ursa/static
	go build -o ./cmd/ursa/static/ursa-bonder ./cmd/ursa-bonder

cmd/ursa/ursa:
	go build -o ./cmd/ursa/ursa -ldflags $(BUILD_LDFLAGS) ./cmd/ursa

clean:
	rm -rf ./cmd/ursa/ursa ./cmd/ursa/static/ursa-bonder

build: cmd/ursa/static/ursa-bonder cmd/ursa/ursa