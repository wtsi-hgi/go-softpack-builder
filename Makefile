PKG := github.com/wtsi-hgi/go-softpack-builder
VERSION := $(shell git describe --tags --always --long --dirty)
TAG := $(shell git describe --abbrev=0 --tags)
LDFLAGS = -ldflags "-X ${PKG}/cmd.Version=${VERSION}"
export GOPATH := $(shell go env GOPATH)
PATH := ${PATH}:${GOPATH}/bin

default: install

# CGO_ENABLED=1 required because unix group lookups no longer work without it

build: export CGO_ENABLED = 0
build:
	go build -tags netgo ${LDFLAGS} -o gsb

install: export CGO_ENABLED = 0
install:
	@rm -f ${GOPATH}/bin/gsb
	@go install -tags netgo ${LDFLAGS}
	@mv ${GOPATH}/bin/go-softpack-builder ${GOPATH}/bin/gsb
	@echo installed to ${GOPATH}/bin/gsb

test: export CGO_ENABLED = 0
test:
	@go test -tags netgo --count 1 .
	@go test -tags netgo --count 1 $(shell go list ./... | tail -n+2)

race: export CGO_ENABLED = 1
race:
	@go test -tags netgo -race --count 1 .
	@go test -tags netgo -race --count 1 $(shell go list ./... | tail -n+2)

# curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.52.2
lint:
	@golangci-lint run

clean:
	@rm -f ./gsb
	@rm -f ./dist.zip

dist: export CGO_ENABLED = 0
# go get -u github.com/gobuild/gopack
# go get -u github.com/aktau/github-release
dist:
	gopack pack --os linux --arch amd64 -o linux-dist.zip
	github-release release --tag ${TAG} --pre-release
	github-release upload --tag ${TAG} --name gsb-linux-x86-64.zip --file linux-dist.zip
	@rm -f gsb linux-dist.zip

.PHONY: test race lint build install clean dist
