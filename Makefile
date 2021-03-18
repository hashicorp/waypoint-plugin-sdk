.PHONY: format
format: # format go code
	gofmt -s -w ./

.PHONY: gen
gen: # generate go code
	go generate .

.PHONY: test
test: # run tests
	go test ./...

