PROTOC_VERSION="3.17.3"

.PHONY: gen
gen: # generate go code
	@# Test for correct version of protoc
	@if [ "$(shell protoc --version | awk '{print $$2}')" != $(PROTOC_VERSION) ]; then \
  		echo "Incorrect version of protoc installed. $(shell protoc --version | awk '{print $2}') detected, $(PROTOC_VERSION) required."; \
  		echo "You can install the correct version from https://github.com/protocolbuffers/protobuf/releases/tag/v$(PROTOC_VERSION) or consider using nix."; \
  		exit 1; \
	 fi

	@# Test for submodule installed
	@test -s "thirdparty/proto/api-common-protos/.git" || { echo "git submodules not initialized, run 'git submodule update --init --recursive' and try again"; exit 1; }

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
	@test -s "thirdparty/proto/api-common-protos/.git" || { echo "git submodules not initialized, run 'git submodule update --init --recursive' and try again"; exit 1; }

	@# Test for protoc installed
	@if [ $(shell which protoc | wc -l) -eq 0 ]; then \
		echo "Required tool protoc not installed." \
		echo "You can install the correct version from https://github.com/protocolbuffers/protobuf/releases/tag/v${PROTOC_VERSION} or consider using nix."; \
		exit 1; \
	 fi

	@echo
	@echo "Done!"


.PHONY: docker/tools
docker/tools: # Creates a docker tools file for generating waypoint server protobuf files
	@echo "Building docker tools image"
	docker build -f tools.Dockerfile -t waypoint-sdk-tools:dev .

.PHONY: docker/gen
docker/gen: docker/tools
	@test -s "thirdparty/proto/api-common-protos/.git" || { echo "git submodules not initialized, run 'git submodule update --init --recursive' and try again"; exit 1; }
	docker run -v `pwd`:/waypoint -it docker.io/library/waypoint-sdk-tools:dev make gen
