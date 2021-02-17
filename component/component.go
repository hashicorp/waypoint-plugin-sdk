// Package component has the interfaces for all the components that
// can be implemented. A component is the broad term used to describe
// all builders, platforms, registries, etc.
//
// Many component interfaces have functions named `XFunc` where "X" is some
// operation and the return value is "interface{}". These functions should return
// a method handle to the function implementing that operation. This pattern is
// done so that we can support custom typed operations that take and return
// full rich types for an operation. We use a minimal dependency-injection
// framework (see internal/mapper) to call these functions.
package component

//go:generate stringer -type=Type -linecomment
//go:generate mockery -all -case underscore

// Type is an enum of all the types of components supported.
// This isn't used directly in this package but is used by other packages
// to reference the component types.
type Type uint

const (
	InvalidType        Type = iota // Invalid
	BuilderType                    // Builder
	RegistryType                   // Registry
	PlatformType                   // Platform
	ReleaseManagerType             // ReleaseManager
	LogPlatformType                // LogPlatform
	LogViewerType                  // LogViewer
	AuthenticatorType              // Authenticator
	MapperType                     // Mapper
	ConfigSourcerType              // ConfigSourcer
	maxType
)

// TypeMap is a mapping of Type to the nil pointer to the interface of that
// type. This can be used with libraries such as mapper.
var TypeMap = map[Type]interface{}{
	BuilderType:        (*Builder)(nil),
	RegistryType:       (*Registry)(nil),
	PlatformType:       (*Platform)(nil),
	ReleaseManagerType: (*ReleaseManager)(nil),
	LogPlatformType:    (*LogPlatform)(nil),
	LogViewerType:      (*LogViewer)(nil),
	AuthenticatorType:  (*Authenticator)(nil),
	ConfigSourcerType:  (*ConfigSourcer)(nil),
}

// Builder is responsible for building an artifact from source.
type Builder interface {
	// BuildFunc should return the method handle for the "build" operation.
	// The build function has access to a *Source and should return an Artifact.
	BuildFunc() interface{}
}

// Registry is responsible for managing artifacts.
type Registry interface {
	// PushFunc should return the method handle to the function for the "push"
	// operation. The push function should take an artifact type and push it
	// to the registry.
	PushFunc() interface{}
}

// Platform is responsible for deploying artifacts.
type Platform interface {
	// DeployFunc should return the method handle for the "deploy" operation.
	// The deploy function has access to the following and should use this
	// as necessary to perform a deploy.
	//
	//   artifact, artifact registry
	//
	DeployFunc() interface{}
}

// PlatformReleaser is an optional interface that a Platform can implement
// to provide default Release functionality. This only takes effect if
// no release is configured.
type PlatformReleaser interface {
	// DefaultReleaserFunc() should return a function that returns
	// a ReleaseManger implementation. This ReleaseManager will NOT have
	// any config so it must work by default.
	DefaultReleaserFunc() interface{}
}

// ReleaseManager is responsible for taking a deployment and making it
// "released" which means that traffic can now route to it.
type ReleaseManager interface {
	// ReleaseFunc should return the method handle for the "release" operation.
	ReleaseFunc() interface{}
}

// Destroyer is responsible for destroying resources associated with this
// implementation. This can be implemented by all of the component types
// and will be called to perform cleanup on any created resources.
type Destroyer interface {
	// DestroyFunc should return the method handle for the destroy operation.
	DestroyFunc() interface{}
}

// Exec is responsible for starting the exec plugin to allow a deployment
// plugin to provide it's own exec functionality. By default, the waypoint exec
// functionality is achieved by creating a session on a long running instance of
// a deployment. But if a platform plugin type does not creat any long running
// instances, they can implement this interface and provide the exec functionality
// in their own bespoke way.
type Execer interface {
	// ExecFunc should return the method handle for a exec session operation.
	// This function has the following types available:
	//  - hclog.Logger
	//  - context.Context
	//  - The Deployment type implemented by the plugin
	//  - *component.ExecSessionInfo
	//  - UI
	//
	// The ExecSessionInfo value contains all the things required launch the
	// exec session.
	ExecFunc() interface{}
}

// ExecResult is returned by an Exec function to indicate the status of the
// run command.
type ExecResult struct {
	// ExitCode is the exit code for the process that was run.
	ExitCode int
}

// WorkspaceDestroyer is called when a workspace destroy operation is
// performed (typically via the "waypoint destroy" CLI). This can be implemented
// by any plugin.
type WorkspaceDestroyer interface {
	// DestroyWorkspaceFunc is called when a workspace destroy operation is performed.
	//
	// This will only be called if that plugin had performed some operation
	// previously on the workspace. This may be called multiple times so it should
	// be idempotent. This will be called after all individual DestroyFuncs are
	// complete.
	DestroyWorkspaceFunc() interface{}
}

// Authenticator is responsible for authenticating different types of plugins.
type Authenticator interface {
	// AuthFunc should return the method for getting credentials for a
	// plugin. This should return AuthResult.
	AuthFunc() interface{}

	// ValidateAuthFunc should return the method for validating authentication
	// credentials for the plugin
	ValidateAuthFunc() interface{}
}

// See Args.Source in the protobuf protocol.
type Source struct {
	App  string
	Path string
}

// AuthResult is the return value expected from Authenticator.AuthFunc.
type AuthResult struct {
	// Authenticated when true means that the plugin should now be authenticated
	// (given the other fields in this struct). If ValidateAuth is called,
	// it should succeed. If this is false, the auth method may have printed
	// help text or some other information, but it didn't authenticate. However,
	// this is not an error.
	Authenticated bool
}

type LabelSet struct {
	Labels map[string]string
}

// JobInfo is available to plugins to get information about the context
// in which a job is executing.
type JobInfo struct {
	// Id is the ID of the job that is executing this plugin operation.
	// If this is empty then it means that the execution is happening
	// outside of a job.
	Id string

	// Local is true if the operation is running locally on a machine
	// alongside the invocation. This can be used to determine if you can
	// do things such as open browser windows, read user files, etc.
	Local bool

	// Workspace is the workspace that this job is executing in. This should
	// be used by plugins to properly isolate resources from each other.
	Workspace string
}

// DeploymentInfo is available to some plugins to get information about the
// deployment.
//
// This is currently only available to ConfigSourcer and provides information
// about the running deployment.
type DeploymentInfo struct {
	// ComponentName is the name of the plugin that launched this deployment.
	ComponentName string

	// Labels of the deployment
	Labels map[string]string
}

type Artifact interface {
	// Labels are the labels to set. These will overwrite any conflicting
	// labels on the value. Please namespace the labels you set. The recommended
	// namespacing is using a URL structure, followed by a slash, and a key.
	// For example: "plugin.example.com/key" as the key. The value can be
	// any string.
	Labels() map[string]string
}

type Deployment interface{}

type Release interface {
	// URL is the URL to access this release.
	URL() string
}

// Template can be implemented by Artifact, Deployment, and Release. This
// will expose this information as available variables in the HCL configuration
// as well as functions in the `template`-prefixed family, such as `templatefile`.
//
// If Template is NOT implemented, we will automatically infer template data
// based on exported variables of the result value. This may not be desirable
// in which case you should implement Template and return nil.
type Template interface {
	// TemplateData returns the template information to make available.
	//
	// This should return empty values for available keys even if the
	// struct it is attached to is nil. This enables the automated documentation
	// generator to work properly. Do not return "nil" when there is no data!
	// And expect that this will be called on nil or empty values.
	TemplateData() map[string]interface{}
}
