.PHONY: gen
gen: # generate go code
	go generate .

.PHONY: format
format: # format go code
	gofmt -s -w ./

.PHONY: test
test: # run tests
	go test ./...
