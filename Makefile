.PHONY: gen
gen: # generate go code
	@test -s "3rdparty/proto/api-common-protos/.git" || { echo "git submodules not initialized, run 'git submodule update --init --recursive' and try again"; exit 1; }
	go generate .

.PHONY: format
format: # format go code
	gofmt -s -w ./

.PHONY: test
test: # run tests
	go test ./...

.PHONY: tools
tools: # install dependencies and tools required to build
	@echo "Fetching tools..."
	go generate -tags tools tools/tools.go
	@echo
	@echo "Done!"

