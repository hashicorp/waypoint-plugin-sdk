.PHONY: test
test: # run tests
	go test ./...

.PHONY: format
format: # format go code
	gofmt -s -w ./
