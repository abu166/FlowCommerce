.PHONY: build 
build:
	go build -o m ./cmd/marketflow/main.go

.DEFAULT_GOAL := build 