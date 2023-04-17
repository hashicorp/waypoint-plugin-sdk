// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package docs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHiddenDocsFields(t *testing.T) {
	require := require.New(t)

	type config struct {
		normal     string `hcl:"normal"`
		deprecated string `hcl:"deprecated" docs:"hidden"` // Hidden docs fields are invisible
	}

	expectedFields := map[string]*FieldDocs{
		"normal": {
			Field: "normal",
			Type:  "string",
		},
	}

	actualFields := make(map[string]*FieldDocs)

	require.Nil(fromConfig(&config{}, actualFields))

	require.Equal(expectedFields, actualFields)
}
