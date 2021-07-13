package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

	"github.com/hashicorp/waypoint-plugin-sdk/internal-shared/protomappers"
	sdkplugin "github.com/hashicorp/waypoint-plugin-sdk/internal/plugin"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/stdio"
)

//go:generate sh -c "protoc -I`go list -m -f \"{{.Dir}}\" github.com/mitchellh/protostructure` -I ./thirdparty/proto/api-common-protos -I proto/ proto/*.proto --go_out=plugins=grpc:proto/gen/"

// Main is the primary entrypoint for plugins serving components. This
// function never returns; it blocks until the program is exited. This should
// be called immediately in main() in your plugin binaries, no prior setup
// should be done.
func Main(opts ...Option) {

	var c config

	// Default our mappers
	c.Mappers = append(c.Mappers, protomappers.All...)

	// Build config
	for _, opt := range opts {
		opt(&c)
	}

	// We have to rewrite the fatih/color package output/error writers
	// to be our plugin stdout/stderr. We use the color package a lot in
	// our UI and this causes the UI to work.
	color.Output = colorable.NewColorable(stdio.Stdout())
	color.Error = colorable.NewColorable(stdio.Stderr())

	// Create our logger. We also set this as the default logger in case
	// any other libraries are using hclog and our plugin doesn't properly
	// chain it along.
	log := hclog.New(&hclog.LoggerOptions{
		Name:   "plugin",
		Level:  hclog.Debug,
		Output: os.Stderr,
		Color:  hclog.AutoColor,

		// Critical that this is JSON-formatted. Since we're a plugin this
		// will enable the host to parse our logs and output them in a
		// structured way.
		JSONFormat: true,
	})
	hclog.SetDefault(log)

	// Build up our mappers
	var mappers []*argmapper.Func
	for _, raw := range c.Mappers {
		// If the mapper is already a argmapper.Func, then we let that through as-is
		m, ok := raw.(*argmapper.Func)
		if !ok {
			var err error
			m, err = argmapper.NewFunc(raw, argmapper.Logger(log))
			if err != nil {
				panic(err)
			}
		}

		mappers = append(mappers, m)
	}

	// Serve
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: sdkplugin.Handshake,
		VersionedPlugins: sdkplugin.Plugins(
			sdkplugin.WithComponents(c.Components...),
			sdkplugin.WithMappers(mappers...),
			sdkplugin.WithLogger(log),
		),
		GRPCServer: plugin.DefaultGRPCServer,
		Logger:     log,
		Test:       c.TestConfig,
	})
}

// config is the configuration for Main. This can only be modified using
// Option implementations.
type config struct {
	// Components is the list of components to serve from the plugin.
	Components []interface{}

	// Mappers is the list of mapper functions.
	Mappers []interface{}

	// TestConfig should only be set when the plugin is being tested; it
	// will opt out of go-plugin's lifecycle management and other features,
	// and will use the supplied configuration options to control the
	// plugin's lifecycle and communicate connection information. See the
	// go-plugin GoDoc for more information.
	TestConfig *plugin.ServeTestConfig
}

// Option modifies config. Zero or more can be passed to Main.
type Option func(*config)

// WithComponents specifies a list of components to serve from the plugin
// binary. This will append to the list of components to serve. You can
// currently only serve at most one of each type of plugin.
func WithComponents(cs ...interface{}) Option {
	return func(c *config) { c.Components = append(c.Components, cs...) }
}

// WithMappers specifies a list of mappers to apply to the plugin.
//
// Mappers are functions that take zero or more arguments and return
// one result (optionally with an error). These can be used to convert argument
// types as needed for your plugin functions. For example, you can convert a
// proto type to a richer Go struct.
//
// Mappers must take zero or more arguments and return exactly one or two
// values where the second return type must be an error. Example:
//
//   func() *Value
//   func() (*Value, error)
//   -- the above with any arguments
//
// This will append the mappers to the list of available mappers. A set of
// default mappers is always included to convert from SDK proto types to
// richer Go structs.
func WithMappers(ms ...interface{}) Option {
	return func(c *config) { c.Mappers = append(c.Mappers, ms...) }
}

// DebugServe starts a plugin server in debug mode; this should only be used
// when the plugin will manage its own lifecycle. It is not recommended for
// normal usage; Serve is the correct function for that.
func DebugServe(ctx context.Context, opts ...Option) (ReattachConfig, <-chan struct{}, error) {
	reattachCh := make(chan *plugin.ReattachConfig)
	closeCh := make(chan struct{})

	opts = append(opts, func(c *config) {
		c.TestConfig = &plugin.ServeTestConfig{
			Context:          ctx,
			ReattachConfigCh: reattachCh,
			CloseCh:          closeCh,
		}
	})

	go Main(opts...)

	var config *plugin.ReattachConfig
	select {
	case config = <-reattachCh:
	case <-time.After(2 * time.Second):
		return ReattachConfig{}, closeCh, fmt.Errorf("timeout waiting on reattach config")
	}

	if config == nil {
		return ReattachConfig{}, closeCh, fmt.Errorf("nil reattach config received")
	}

	return ReattachConfig{
		Protocol:        string(config.Protocol),
		ProtocolVersion: config.ProtocolVersion,
		Pid:             config.Pid,
		Test:            config.Test,
		Addr: ReattachConfigAddr{
			Network: config.Addr.Network(),
			String:  config.Addr.String(),
		},
	}, closeCh, nil
}

// Debug starts a debug server and controls its lifecycle, printing the
// information needed for Waypoint to connect to the plugin to stdout.
// os.Interrupt will be captured and used to stop the server.
func Debug(ctx context.Context, pluginAddr string, opts ...Option) error {
	ctx, cancel := context.WithCancel(ctx)
	// Ctrl-C will stop the server
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer func() {
		signal.Stop(sigCh)
		cancel()
	}()
	config, closeCh, err := DebugServe(ctx, opts...)
	if err != nil {
		return fmt.Errorf("Error launching debug server: %w", err)
	}
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	reattachBytes, err := json.Marshal(map[string]ReattachConfig{
		pluginAddr: config,
	})
	if err != nil {
		return fmt.Errorf("Error building reattach string: %w", err)
	}

	reattachStr := string(reattachBytes)

	fmt.Printf("Plugin started, to attach Waypoint set the WP_REATTACH_PLUGINS env var:\n\n")
	switch runtime.GOOS {
	case "windows":
		fmt.Printf("\tCommand Prompt:\tset \"WP_REATTACH_PLUGINS=%s\"\n", reattachStr)
		fmt.Printf("\tPowerShell:\t$env:WP_REATTACH_PLUGINS='%s'\n", strings.ReplaceAll(reattachStr, `'`, `''`))
	case "linux", "darwin":
		fmt.Printf("\tWP_REATTACH_PLUGINS='%s'\n", strings.ReplaceAll(reattachStr, `'`, `'"'"'`))
	default:
		fmt.Println(reattachStr)
	}
	fmt.Println("")

	// wait for the server to be done
	<-closeCh
	return nil
}

// ReattachConfig holds the information Waypoint needs to be able to attach
// itself to a plugin process, so it can drive the process.
type ReattachConfig struct {
	Protocol        string
	ProtocolVersion int
	Pid             int
	Test            bool
	Addr            ReattachConfigAddr
}

// ReattachConfigAddr is a JSON-encoding friendly version of net.Addr.
type ReattachConfigAddr struct {
	Network string
	String  string
}
