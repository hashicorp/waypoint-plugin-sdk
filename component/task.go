package component

// TaskLaunchInfo is used by TaskLauncher's StartTaskFunc operation.
// This type provides the details about how the new task should be configured.
type TaskLaunchInfo struct {
	// OciUrl is a docker-run compatible specifier for an OCI image. For instance,
	// it supports the bare types like `ubuntu`, as well as toplevel versioned
	// types like `ubuntu:latest`, and any values that contain fully qualified
	// hostnames like `docker.io/library/ubuntu:latest`.
	OciUrl string

	// EnvironmentVariables is the set of variables that should be configured
	// for the task to see when it starts.
	EnvironmentVariables map[string]string

	// Entrypoint is the entrypoint override for this image. If this is not
	// set, then the default entrypoint for the OCI image should be used.
	Entrypoint []string

	// Arguments is passed as command line arguments to the image when it started.
	// If the image defines an entrypoint, then the arguments will be passed as
	// arguments to that program.
	Arguments []string
}
