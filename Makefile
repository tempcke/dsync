project = $(shell basename $(shell pwd))
MODULE = $(shell go list -m)

help:			## display help information
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

lint:			## lint
	test -z $(shell go fmt ./...)
	go vet ./...
	staticcheck ./...

updateDepts:	## update go module dependencies
	go get -u ./...
	go mod tidy
	go mod vendor

test:			## run tests
	time -p go test -failfast -timeout=30s ./... | grep -v '\[no test'

testAll: lint	## run tests no cache
	time -p go test -failfast -timeout=30s -count=1 -race ./... | grep -v '\[no test'
	govulncheck ./...

.PHONY: help lint updateDepts test testAll