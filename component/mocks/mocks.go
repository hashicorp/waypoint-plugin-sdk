// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package mocks

import (
	"reflect"

	"github.com/stretchr/testify/mock"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
)

// ForType returns an implementation of the given type that supports mocking.
func ForType(t component.Type) interface{} {
	// Note that the tests in mocks_test.go verify that we support all types
	switch t {
	case component.BuilderType:
		return &Builder{}

	case component.RegistryType:
		return &Registry{}

	case component.PlatformType:
		return &Platform{}

	case component.LogPlatformType:
		return &LogPlatform{}

	case component.ReleaseManagerType:
		return &ReleaseManager{}

	case component.AuthenticatorType:
		return &Authenticator{}

	case component.ConfigSourcerType:
		return &ConfigSourcer{}

	case component.TaskLauncherType:
		return &TaskLauncher{}

	default:
		return nil
	}
}

// Mock returns the Mock field for the given interface. The interface value
// should be one of the mocks in this package. This will panic if an incorrect
// value is given, error checking is not done.
func Mock(v interface{}) *mock.Mock {
	value := reflect.ValueOf(v)
	if value.Kind() == reflect.Interface {
		value = reflect.Indirect(value)
	}
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	field := value.FieldByName("Mock")
	return field.Addr().Interface().(*mock.Mock)
}
