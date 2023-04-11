//go:build tools
// +build tools

// To install the following tools at the version used by this repo run:
// $ make tools
// or
// $ go generate -tags tools tools/tools.go

package tools

//go:generate go install google.golang.org/protobuf/cmd/protoc-gen-go
//go:generate go install google.golang.org/grpc/cmd/protoc-gen-go-grpc

import (
	//go:generate go install google.golang.org/protobuf/cmd/protoc-gen-go
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"

	//go:generate go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
)
