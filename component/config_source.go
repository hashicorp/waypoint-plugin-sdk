package component

// ConfigSourcer can be implemented by plugins that support sourcing
// dynamic configuration for running applications.
//
// This plugin type runs alongside the application. The initial config loading
// will block the application start so authors should attempt to source
// configuration as quickly as possible.
type ConfigSourcer interface {
	// ReadFunc returns the function for reading configuration.
	//
	// The returned function can start a background goroutine to more efficiently
	// watch for changes. The entrypoint will periodically call Read to check for
	// updates.
	//
	// If the configuration changes for any dynamic configuration variable,
	// the entrypoint will call Stop followed by Read, so plugins DO NOT need
	// to implement config diffing. Plugins may safely assume if Read is called
	// after a Stop that the config is new, and that subsequent calls have the
	// same config.
	//
	// Read is called for ALL defined configuration variables for this source.
	// If ANY change, Stop is called followed by Read again. Only one sourcer
	// is active for a set of configs.
	ReadFunc() interface{}

	// StopFunc returns a function for stopping configuration sourcing.
	// You can return nil if stopping is not necessary or supported for
	// this sourcer.
	//
	// The stop function should stop any background processes started with Read.
	StopFunc() interface{}
}

// ConfigRequest is sent to ReadFunc for ConfigSourcer to represent a
// single configuration variable that was requested. The ReadFunc parameters
// should have a `[]*ConfigRequest` parameter for these.
type ConfigRequest struct {
	Name   string
	Config map[string]string
}
