package resource

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/opaqueany"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// Manager manages the lifecycle and state of one or more resources.
//
// A resource manager makes it easy to define the set of resources you want
// to create, create them, handle any errors, and persist the state of the
// resources you've created (such as IDs or other metadata) so that you can
// update or destroy the resources later.
//
// Create a Manager with NewManager and a set of options.
type Manager struct {
	resources      map[string]*Resource
	createState    *createState
	logger         hclog.Logger
	valueProviders []interface{}
	dcr            *component.DeclaredResourcesResp
	dtr            *component.DestroyedResourcesResp
}

// NewManager creates a new resource manager.
//
// Callers should call Validate on the result to check for errors.
func NewManager(opts ...ManagerOption) *Manager {
	var m Manager
	m.resources = map[string]*Resource{}
	m.logger = hclog.L()
	for _, opt := range opts {
		opt(&m)
	}
	return &m
}

// Validate checks that the manager and all the resources that are part
// of this manager are configured correctly. This will always be called
// prior to any lifecycle operation, but users may call this earlier to
// better control when this happens.
func (m *Manager) Validate() error {
	var result error

	// Validate each resource
	for _, r := range m.resources {
		err := r.Validate()
		if err == nil {
			continue
		}

		// We prefix all the error messages with the resource name so
		// that users can better identify them.
		prefix := r.name
		if prefix == "" {
			prefix = "unnamed resource"
		}
		err = multierror.Prefix(err, prefix+": ")

		result = multierror.Append(result, err)
	}

	return result
}

// Resource returns the resource with the given name. This will return nil
// if the resource is not known.
func (m *Manager) Resource(n string) *Resource {
	return m.resources[n]
}

// LoadState loads the serialized state from Proto.
func (m *Manager) LoadState(v *opaqueany.Any) error {
	var s pb.Framework_ResourceManagerState
	if err := component.ProtoAnyUnmarshal(v, &s); err != nil {
		return err
	}

	// Initialize our creation state from the serialized state
	m.createState = &createState{Order: s.CreateOrder}

	// Go through each resource and populate their state
	for _, sr := range s.Resources {
		r, ok := m.resources[sr.Name]
		if !ok {
			return fmt.Errorf(
				"failed to deserialize state: unknown resource %q", sr.Name)
		}

		if err := r.loadState(sr); err != nil {
			return err
		}
	}

	return nil
}

// State returns the serialized state for this manager and all the resources
// that are part of this manager. This is a `google.protobuf.Any` type and
// plugin authors are expected to serialize this type directly into their
// return values. This is an opaque type; plugin authors should make no attempt
// to deserialize this.
func (m *Manager) State() *opaqueany.Any {
	result, err := component.ProtoAny(m.proto())
	if err != nil {
		// This should never happen. Errors that happen are usually encoded
		// into the state as messages or a panic occurs if it is critical.
		// We don't expect this to ever panic because Validate should test
		// this.
		panic(err)
	}

	return result
}

func (m *Manager) proto() *pb.Framework_ResourceManagerState {
	var result pb.Framework_ResourceManagerState
	for _, r := range m.resources {
		result.Resources = append(result.Resources, r.proto())
	}

	// If we have creation station, then track the order. We will use
	// this to construct the destroy order later.
	if cs := m.createState; cs != nil {
		result.CreateOrder = cs.Order
	}

	return &result
}

// CreateAll creates all the resources for this manager.
//
// The ordering will be determined based on the creation function dependencies
// for each resource.
//
// Create will initialize brand new state. This will not reuse existing state.
// If there is any existing state loaded, this will return an error immediately
// because it risks that state being lost.
func (m *Manager) CreateAll(args ...interface{}) error {
	if err := m.Validate(); err != nil {
		return err
	}

	// We need to build up the final function in our argmapper chain. This
	// function will do nothing, but will take as an input all the marker
	// values for the resources we want to create. This will force argmapper
	// to call all our create functions for all our resources.
	finalInputs := make([]argmapper.Value, 0, len(m.resources))
	for _, r := range m.resources {
		finalInputs = append(finalInputs, markerValue(r.name))
	}

	finalInputSet, err := argmapper.NewValueSet(finalInputs)
	if err != nil {
		return err
	}

	finalFunc, err := argmapper.BuildFunc(
		finalInputSet, nil,
		func(in, out *argmapper.ValueSet) error {
			// no-op on purpose. This function only exists to set the
			// required inputs for argmapper to create the correct call
			// graph.
			return nil
		},
	)
	if err != nil {
		return err
	}

	// Reset our creation state if we're creating
	m.createState = &createState{}

	// Start building our arguments
	mapperArgs, err := m.mapperArgs()
	if err != nil {
		return err
	}
	for _, arg := range args {
		mapperArgs = append(mapperArgs, argmapper.Typed(arg))
	}
	for _, r := range m.resources {
		createFunc, err := r.mapperForCreate(m.createState)
		if err != nil {
			return err
		}

		mapperArgs = append(mapperArgs, argmapper.ConverterFunc(createFunc))
	}

	result := finalFunc.Call(mapperArgs...)

	// If we got an error, perform an automatic rollback.
	resultErr := result.Err()
	if resultErr != nil {
		m.logger.Info("error during creation, starting rollback", "err", resultErr)
		if err := m.DestroyAll(args...); err != nil {
			m.logger.Warn("error during rollback", "err", err)
			resultErr = multierror.Append(resultErr, fmt.Errorf(
				"Error during rollback: %w", err))
		} else {
			m.logger.Info("rollback successful")
		}
	}

	// Now that resource state has been filled, populate the declared resource response if available.
	if m.dcr != nil {
		for name, resource := range m.resources {
			declaredResource, err := resource.DeclaredResource()
			if err != nil {
				// Will likely only occur when developing plugins.
				m.logger.Debug("Failed to generate declared resource",
					"resource name", name,
					"platform", resource.platform,
					"error", err,
				)
				continue
			}
			m.dcr.DeclaredResources = append(m.dcr.DeclaredResources, declaredResource)
		}
	}

	return resultErr
}

// DestroyAll destroys all the resources under management. This will call
// Destroy in the reverse order of Create. All the state that was created
// via Create will be available to the Destroy callbacks. Note that after
// a resource is destroyed, their state is also set to nil.
//
// Only resources that have been created will be destroyed. This means
// that if Create partially failed, then only the resources that attempted
// creation will have Destroy called. Resources that were never called to
// Create will do nothing.
func (m *Manager) DestroyAll(args ...interface{}) error {
	if err := m.Validate(); err != nil {
		return err
	}

	cs := m.createState
	if cs == nil || len(cs.Order) == 0 {

		// If we have no creation order, then we fall back to checking
		// manually for state set on each resource. Note this has a huge
		// limitation in that our Order is probably wrong. For the case we're
		// implementing this for, the order doesn't matter so this works,
		// and hopefully by the time ordering matters everything is swapped
		// over to the resource manager.
		for n, r := range m.resources {
			if r.State() == nil {
				continue
			}

			// We have state, so we want to destroy this.
			if cs == nil {
				cs = &createState{}
			}

			cs.Order = append(cs.Order, n)
		}

		// Still empty? Then we do nothing
		if cs == nil || len(cs.Order) == 0 {
			return nil
		}

		// We need to sort the order by the setStateClocks on the resources
		// since for the manual case, we expect users to call SetState in creation
		// order.
		sort.Slice(cs.Order, func(i, j int) bool {
			ir, jr := m.resources[cs.Order[i]], m.resources[cs.Order[j]]
			return ir.setStateClock < jr.setStateClock
		})
	}

	var finalInputs []argmapper.Value
	mapperArgs, err := m.mapperArgs()
	if err != nil {
		return err
	}
	for _, arg := range args {
		mapperArgs = append(mapperArgs, argmapper.Typed(arg))
	}

	// Go through our creation order and create all our destroyers.
	for i := 0; i < len(cs.Order); i++ {
		r := m.Resource(cs.Order[i])
		if r == nil {
			// We are missing a resource that we should be destroying.
			return fmt.Errorf(
				"destroy failed: missing resource definition %q",
				cs.Order[i],
			)
		}

		// The dependencies are the resources that were created after
		// this resource.
		var deps []string
		if next := i + 1; next < len(cs.Order) {
			deps = cs.Order[next:]
		}

		// Create the mapper for destroy. The dependencies are the set of
		// created resources in the creation order that were ahead of this one.
		f, err := r.mapperForDestroy(deps)
		if err != nil {
			return err
		}
		mapperArgs = append(mapperArgs,
			argmapper.ConverterFunc(f),
			argmapper.Typed(r.State()),
		)

		// Ensure that our final func is dependent on the marker for
		// this resource so that it definitely gets called.
		finalInputs = append(finalInputs, markerValue(r.name))
	}

	// Create our final target function. This has as dependencies all the
	// markers for the resources that should be destroyed.
	finalInputSet, err := argmapper.NewValueSet(finalInputs)
	if err != nil {
		return err
	}

	finalFunc, err := argmapper.BuildFunc(
		finalInputSet, nil,
		func(in, out *argmapper.ValueSet) error {
			// no-op on purpose. This function only exists to set the
			// required inputs for argmapper to create the correct call
			// graph.
			return nil
		},
	)
	if err != nil {
		return err
	}

	// Call it
	result := finalFunc.Call(mapperArgs...)

	resultErr := result.Err()
	if resultErr != nil {
		m.logger.Info("error during destruction", "err", resultErr)
	} else {
		// If this was successful, then we clear out our creation state.
		m.createState = nil
	}

	// Populate the declared/destroyed resources. The declared resources are the resources
	// which remain after destroying, and the destroyed resources are the ones that have
	// been destroyed (which implement WithDestroy). If a resource does not implement a
	// destroy function, then it is a declaredResource. If it does, it's a destroyedResource
	if m.dcr != nil || m.dtr != nil {
		for name, resource := range m.resources {
			if resource.destroyFunc != nil {
				destroyedResource, err := resource.DestroyedResource()
				if err != nil {
					m.logger.Debug("Failed to convert resource to a DestroyedResource proto message",
						"resource name", name,
						"error", err,
					)
					return err
				}

				m.dtr.DestroyedResources = append(m.dtr.DestroyedResources, destroyedResource)
			} else {
				declaredResource, err := resource.DeclaredResource()
				if err != nil {
					m.logger.Debug("Failed to convert resource to a DeclaredResource proto message",
						"resource name", name,
						"error", err,
					)
					return err
				}
				m.dcr.DeclaredResources = append(m.dcr.DeclaredResources, declaredResource)
			}
		}
	}

	return result.Err()
}

// healthSummary figures out what the overall health and message should be for a given set of resources.
// If all resources return the same health, the overall health will be that health. If resources
// return different healths, the overall health will be PARTIAL, and the health message
// will give more details.
func healthSummary(resources []*pb.StatusReport_Resource) (
	overallHealth pb.StatusReport_Health, overallHealthMessage string, err error,
) {
	if len(resources) == 0 {
		return overallHealth, overallHealthMessage, fmt.Errorf("cannot evaluate health summary - no resources provided")
	}

	// count healths by overall health, then by resource type
	// Key health type by int so we can sort by the order the types are defined in the protobuf definition
	countByHealthByResourceType := make(map[int]map[string]int)
	for _, r := range resources {
		if r == nil {
			return overallHealth, overallHealthMessage, fmt.Errorf("cannot evaluate health summary with a nil resource")
		}
		if _, ok := countByHealthByResourceType[int(r.Health)]; !ok {
			countByHealthByResourceType[int(r.Health)] = make(map[string]int)
		}
		healthsByResource := countByHealthByResourceType[int(r.Health)]
		healthsByResource[r.Type]++
	}

	// We want to generate healths in a deterministic order. To do that,
	// we need to pull out the map keys, sort them, and access by those in order.
	var distinctHealths []int
	for k := range countByHealthByResourceType {
		distinctHealths = append(distinctHealths, k)
	}

	if len(distinctHealths) == 1 {
		return pb.StatusReport_Health(distinctHealths[0]),
			fmt.Sprintf("All %d resources are reporting %s", len(resources), pb.StatusReport_Health_name[int32(distinctHealths[0])]),
			nil
	}

	// We have more than one distinct health type, we have some kind of partial status
	overallHealth = pb.StatusReport_PARTIAL

	sort.Ints(distinctHealths)
	for _, healthStatus := range distinctHealths {
		countByResourceType := countByHealthByResourceType[healthStatus]

		var distinctResourceTypes []string
		for k := range countByResourceType {
			distinctResourceTypes = append(distinctResourceTypes, k)
		}
		sort.Strings(distinctResourceTypes)

		for _, resourceType := range distinctResourceTypes {
			count := countByResourceType[resourceType]
			overallHealthMessage = overallHealthMessage + fmt.Sprintf("%d %s %s, ", count, resourceType, pb.StatusReport_Health_name[int32(healthStatus)])
		}
	}
	overallHealthMessage = strings.TrimSuffix(overallHealthMessage, ", ")

	return overallHealth, overallHealthMessage, nil
}

// StatusReport generates a report by invoking the statusFunc method of all resources under management,
// using those statuses to determine an overall composite health, and returns it within a
// status report.
// If all resources return the same health, the overall health will be that health. If resources
// return different healths, the overall health will be PARTIAL, and the health message
// will give more details.
// If your plugin wishes to use a different algorithm for determining overall health, you may
// modify this report before returning from your status function.
func (m *Manager) StatusReport(args ...interface{}) (*pb.StatusReport, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	resources, err := m.StatusAll(args...)
	if err != nil {
		return nil, fmt.Errorf("failed generating resource statuses: %s", err)
	}

	// Determine overall health based on these resources
	health, healthMessage, err := healthSummary(resources)
	if err != nil {
		return nil, err
	}

	return &pb.StatusReport{
		External:      true,
		GeneratedTime: timestamppb.Now(),
		Resources:     resources,
		Health:        health,
		HealthMessage: healthMessage,
	}, nil
}

// StatusAll invokes the statusFunc method of all the resources under management.
// The order in which the status of each resource is queried is
// non-deterministic, and does rely on any creation order or state of the
// resource. All the state that was created via Create will be available to the
// Status callbacks, if any. Resources are not required to have a state to have
// a status. Returns a slice of reports or an error.
func (m *Manager) StatusAll(args ...interface{}) ([]*pb.StatusReport_Resource, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	mapperArgs, err := m.mapperArgs()
	if err != nil {
		return nil, err
	}
	for _, arg := range args {
		mapperArgs = append(mapperArgs, argmapper.Typed(arg))
	}

	var finalInputs []argmapper.Value
	// Go through available resources.
	for _, r := range m.resources {
		// Create the mapper for status
		f, err := r.mapperForStatus()
		if err != nil {
			return nil, err
		}
		mapperArgs = append(mapperArgs,
			argmapper.ConverterFunc(f),
			// the status methods should receive the resource state, if any
			// exists
			argmapper.Typed(r.State()),
		)

		// Ensure that our final func is dependent on the marker for
		// this resource so that it definitely gets called.
		finalInputs = append(finalInputs, markerValue(r.name))
	}

	// Create our final target function.
	finalInputSet, err := argmapper.NewValueSet(finalInputs)
	if err != nil {
		return nil, err
	}

	finalFunc, err := argmapper.BuildFunc(
		finalInputSet, nil,
		func(in, out *argmapper.ValueSet) error {
			// no-op on purpose. This function only exists to set the
			// required inputs for argmapper to create the correct call
			// graph.
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	// Call it
	result := finalFunc.Call(mapperArgs...)
	if result.Err() != nil {
		return nil, result.Err()
	}
	var reports []*pb.StatusReport_Resource
	for _, r := range m.resources {
		if st := r.Status(); st != nil {
			// Fill in the declared resource ref for each resource the plugin made.
			for _, stResource := range st.Resources {
				stResource.DeclaredResource = &pb.Ref_DeclaredResource{Name: r.name}
			}
			reports = append(reports, st.Resources...)
		}
	}
	return reports, nil
}

func (m *Manager) mapperArgs() ([]argmapper.Arg, error) {
	result := []argmapper.Arg{
		argmapper.Logger(m.logger),
	}

	// Add our value providers which are always available
	for _, raw := range m.valueProviders {
		f, err := argmapper.NewFunc(raw, argmapper.FuncOnce())
		if err != nil {
			return nil, err
		}

		result = append(result, argmapper.ConverterFunc(f))
	}

	return result, nil
}

// ManagerOption is used to configure NewManager.
type ManagerOption func(*Manager)

// WithLogger specifies the logger to use. If this is not set then this
// will use the default hclog logger.
func WithLogger(l hclog.Logger) ManagerOption {
	return func(m *Manager) { m.logger = l }
}

// WithResource specifies a resource for the manager. This can be called
// multiple times and the resources will be appended to the manager.
func WithResource(r *Resource) ManagerOption {
	return func(m *Manager) {
		name := r.name

		// If we have no name set, this is an error that will be caught
		// during validation. For now, we generate a ULID so that we can
		// store the resource.
		if name == "" {
			name, _ = component.Id()
		}

		// Note(izaak): If multiple resources have the same name, all but one
		// will be overwritten. We could enforce uniqueness here, but we'd have
		// to introduce an error return.
		m.resources[name] = r
	}
}

// WithValueProvider specifies a function that can provide values for
// the arguments for resource lifecycle functions. This is useful for example
// to setup an API client. The value provider will be called AT MOST once
// for a set of resources (but may be called zero times if no resources
// depend on the value it returns).
//
// The argument f should be a function. The function may accept arguments
// from any other value providers as well.
func WithValueProvider(f interface{}) ManagerOption {
	// NOTE(mitchellh): In the future, we can probably do something fancier
	// here so that if any values returned by this implement io.Closer we will
	// call it or something so we can automatically do resource cleanup. We
	// don't need this today but I can see that being useful.
	return func(m *Manager) {
		m.valueProviders = append(m.valueProviders, f)
	}
}

// WithDeclaredResourcesResp specifies a declared resource response that
// ResourceManager will automatically populate after creating resources. It will
// add one DeclaredResource per resource under management. For most plugins,
// this will be their only interaction with the DeclaredResourcesResponse.
func WithDeclaredResourcesResp(dcr *component.DeclaredResourcesResp) ManagerOption {
	return func(m *Manager) {
		m.dcr = dcr
	}
}

// WithDestroyedResourcesResp specifies a destroyed resource response that
// ResourceManager will automatically populate after creating resources. It will
// add one DestroyedResource per resource being destroyed. For most plugins,
// this will be their only interaction with the DeclaredResourcesResponse.
func WithDestroyedResourcesResp(dtr *component.DestroyedResourcesResp) ManagerOption {
	return func(m *Manager) {
		m.dtr = dtr
	}
}
