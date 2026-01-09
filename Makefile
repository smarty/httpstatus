#!/usr/bin/make -f

test: fmt
	go test -timeout=1s -short -race -covermode=atomic ./...

fmt:
	go fmt ./...

compile:
	go build ./...

build: test compile

.PHONY: test fmt compile build
