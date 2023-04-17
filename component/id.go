// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package component

import (
	"crypto/rand"

	"github.com/oklog/ulid"
)

var ulidReader = ulid.Monotonic(rand.Reader, 1)

// Id returns a unique Id that can be used for new values. This generates
// a ulid value but the ID itself should be an internal detail. An error will
// be returned if the ID could be generated.
func Id() (string, error) {
	id, err := ulid.New(ulid.Now(), ulidReader)
	if err != nil {
		return "", err
	}

	return id.String(), nil
}
