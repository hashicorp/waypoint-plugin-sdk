// Package resource contains helpers around managing groups of resources. A
// "resource" is any element created by a plugin. For example, an AWS
// deployment plugin may create load balancers, security groups, VPCs, etc. Each
// of these is a "resource".
//
// This package simplifies resource lifecycle, state management, error handling,
// and more. Even if your plugin only has a single resource, this library is
// highly recommended.
package resource
