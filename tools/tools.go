// +build tools

// To install the following tools at the version used by this repo run:
// $ make tools
// or
// $ go generate -tags tools tools/tools.go

package tools

//go:generate go install github.com/golang/protobuf/proto
import _ "github.com/golang/protobuf/proto"

//go:generate go install github.com/golang/protobuf/protoc-gen-go
import _ "github.com/golang/protobuf/protoc-gen-go"
