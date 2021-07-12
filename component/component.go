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

import proto "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"

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
	AuthenticatorType              // Authenticator
	MapperType                     // Mapper
	ConfigSourcerType              // ConfigSourcer
	TaskLauncherType               // TaskLauncher
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
	AuthenticatorType:  (*Authenticator)(nil),
	ConfigSourcerType:  (*ConfigSourcer)(nil),
	TaskLauncherType:   (*TaskLauncher)(nil),
}

// TaskLauncher launches a batch task, ie a task that runs to completion and does
// not restart.
type TaskLauncher interface {
	// StartTaskFunc should return a method for the "start task" operation.
	// This will have TaskLaunchInfo available to it to understand what the task
	// should do.
	StartTaskFunc() interface{}

	// StopTaskFunc is called to force a previously started task to stop. It will
	// be passed the state value returned by StartTaskFunc for identification.
	StopTaskFunc() interface{}
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

// LogPlatform is responsible for starting the logs plugin that allows a plugin
// to read logs for a deployment in its own way.
// This function has the following types available:
type LogPlatform interface {
	// LogsFunc should return the method handle for a logs operation.
	// This function has the following types available:
	//  - hclog.Logger
	//  - context.Context
	//  - The Deployment type implemented by the plugin
	//  - *component.LogViewer
	//  - UI
	LogsFunc() interface{}
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

// A DeploymentWithUrl is a Deployment that can be referenced
// with an URL without using any external URL service (like
// Hashicorp Horizon). This means that the platform that
// hosts the workload can provide automatically an URL
// that is publicly (or company-internally)
// reachable.
//
// Example: automatic routing of Kubernetes pods with
// an operator, Cloud Foundry's URLs, ...
type DeploymentWithUrl interface {
	URL() string
}

type Release interface {
	// URL is the URL to access this release.
	URL() string
}

// StatusReport can be implemented by Platform and PlatformReleaser to query
// the target platform and build a report of the current deployments health. If
// this isn't implemented, no status report will be built for the deployments.
type Status interface {
	// StatusReportFunc should return a proto.StatusReport that details the
	// result of the most recent health check for a deployment.
	StatusFunc() interface{}
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

// Generation can be implemented by Platform and PlatformReleaser to explicitly
// specify a "generation" for a deploy or release. If this isn't implemented,
// Waypoint generates a random new generation per operation and assumes
// immutable behavior.
//
// A "generation" specifies a unique identifier to the physical resources used
// by that operation. Two  operations with the same generation are operating
// on the same underlying resources. This is used by Waypoint to detect mutable
// vs immutable  operations; if two operations change generations, then Waypoint
// knows the operation created new resources rather than mutating old ones.
//
// Waypoint uses this information to alter its behavior slightly. For example:
//
//  - a user can only release a generation that isn't already released.
//  - deployment URLs are identical for matching generations
//  - certain functionality in the future such as canaries will use this
//
type Generation interface {
	// GenerationFunc should return the method handle for a function that
	// returns a `[]byte` result (and optionally an error). The `[]byte` is
	// the unique generation for the operation.
	//
	// The returned function will have access to all of the same parameters
	// as the operation function itself such as Deploy or Release.
	GenerationFunc() interface{}
}



// RunningTask is returned from StartTask. It contains the state the plugin can
// use later to stop the task.
type RunningTask interface{}

// OutParameter is an argument type that is used by plugins as a vehicle for returning
// data to core. A struct implementing this interface that indicates to the plugin system
// that the struct should not be included in a grpc advertised dynamic function spec, because
// it will be injected on the plugin side, not supplied from core over GRPC.
type OutParameter interface {
	IsOutParameter()
}

// DeclaredResourcesResp is a component used as a vehicle for plugins to communicate
// the resources that they declare back to core - an "OutParameter". It can be
// accepted as an argument to a Platform's Deploy function, and any DeclaredResources
// added to it will be displayed on the Deployment api.
type DeclaredResourcesResp struct {
	// Resources that a plugin declares have been created and are under its management.
	DeclaredResources []*proto.DeclaredResource
}

// IsOutParameter causes DeclaredResourcesResp to implement the OutParameter interface, which
// will prevent it from being added as an arg to any plugin advertised dynamic function spec.
func (d *DeclaredResourcesResp) IsOutParameter() {}
