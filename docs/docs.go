package docs

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Details documents highlevel information about a plugin.
type Details struct {
	// Description is the highlevel description of the plugin.
	Description string

	// Example is typically an hcl configuration snippit of using
	// the plugin.
	Example string

	// Input is the type of the value that the plugin accepts from the
	// previous plugin. This can be empty if the plugin does not use
	// any inputs.
	Input string

	// Output is the type of the value that the plugin outputs.
	Output string

	// Mappers is the list of mappers that the plugin makes available for
	// type conversion.
	Mappers []Mapper
}

// Mapper indicates the available mappers and what types they convert
// from and to.
type Mapper struct {
	// Input is the type of the value that the mapper takes as input.
	Input string

	// Output is the type of the value that the mapper outputs.
	Output string

	// Description is a simple explanation of how the mapper converts the values.
	Description string
}

// FieldDocs documents a specific attribute the plugin has available.
type FieldDocs struct {
	// Field is the name of the attribute
	Field string

	// Type is the hcl type of the attribute (int, string, etc)
	Type string

	// Synopsis is a short, one line description of the attribute
	Synopsis string

	// Summary is a longer, more indepth description of the attribute.
	Summary string

	// Optional indicates of the attribute is optional or not.
	Optional bool

	// Default indicates the value of the attribute if the user does not set it.
	Default string

	// EnvVar indicates the operating system environment variable that will be
	// used to read the value from if the user does not set it.
	EnvVar string

	// Category indicates that this is not a field itself but a HCL block
	// that has fields underneith it.
	Category bool

	// SubFields is defined when this field is a category. It is the fields
	// in that category.
	SubFields []*FieldDocs

	discoveredFields map[string]*FieldDocs
}

// Documentation allows a plugin to document it's many wonderful features.
type Documentation struct {
	description    string
	example        string
	input          string
	output         string
	fields         map[string]*FieldDocs
	templateFields map[string]*FieldDocs
	requestFields  map[string]*FieldDocs
	mappers        []Mapper
}

// Option is implemented by various functions to automatically populate
// the Documentation.
type Option func(*Documentation) error

// New creates a new Documentation value.
func New(opts ...Option) (*Documentation, error) {
	var d Documentation
	d.fields = make(map[string]*FieldDocs)
	d.templateFields = make(map[string]*FieldDocs)
	d.requestFields = make(map[string]*FieldDocs)

	for _, opt := range opts {
		err := opt(&d)
		if err != nil {
			return nil, err
		}
	}

	return &d, nil
}

// FromConfig populates the Documentation value by reading the struct
// members on the value. This is typically passed the value that is returned
// by the plugin's Config function.
func FromConfig(v interface{}) Option {
	return func(d *Documentation) error {
		return fromConfig(v, d.fields)
	}
}

// RequestFromStruct populates the Documentation's request information
// by reading the struct members on the value. Request information is
// configuration defined by a Config Sourcer to be used as authentication
// and other non-config information.
func RequestFromStruct(v interface{}) Option {
	return func(d *Documentation) error {
		return fromConfig(v, d.requestFields)
	}
}

func fromConfig(v interface{}, target map[string]*FieldDocs) error {
	rv := reflect.ValueOf(v).Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("invalid config type, must be struct")
	}

	t := rv.Type()

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, ok := f.Tag.Lookup("hcl")
		if !ok {
			return fmt.Errorf("missing hcl tag on field: %s", f.Name)
		}

		parts := strings.Split(name, ",")

		if parts[0] == "" {
			continue
		}

		field := &FieldDocs{
			Field: parts[0],
			Type:  cleanupType(f.Type.String()),
		}

		for _, p := range parts[1:] {
			if p == "optional" {
				field.Optional = true
			}
		}

		t := f.Type

		for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice {
			t = t.Elem()
		}

		if t.Kind() == reflect.Struct {
			v := reflect.New(t)

			target := make(map[string]*FieldDocs)

			err := fromConfig(v.Interface(), target)
			if err != nil {
				return err
			}

			field.discoveredFields = target
		}

		target[parts[0]] = field
	}

	return nil
}

func formatHelp(lines ...string) string {
	var sb strings.Builder

	for i, line := range lines {
		if i > 0 {
			sb.WriteByte('\n')
		}

		sb.WriteString(strings.TrimSpace(line))
	}

	return sb.String()
}

type (
	// SummaryString sets the Summary of the field
	SummaryString string

	// Default sets the Default of the field
	Default string

	// EnvVar sets the EnvVar of the field
	EnvVar string

	Category bool
)

type docOption interface {
	docOption() bool
}

func (o SummaryString) docOption() bool { return true }
func (o Default) docOption() bool       { return true }
func (o EnvVar) docOption() bool        { return true }
func (o Category) docOption() bool      { return true }

// Summary creates a SummaryString by doing some light space editting
// and joining of the given array of strings. This is a convienence function
// for writing multi-line summaries that format well as Go code.
func Summary(in ...string) SummaryString {
	var sb strings.Builder

	for i, str := range in {
		if str == "" {
			sb.WriteByte('\n')
		}

		if i > 0 {
			sb.WriteByte(' ')
		}

		sb.WriteString(strings.TrimSpace(str))
	}

	return SummaryString(sb.String())

}

// Example sets the Example field of the Documentation
func (d *Documentation) Example(x string) {
	d.example = x
}

// Description sets the Description field of the Documentation
func (d *Documentation) Description(x string) {
	d.description = x
}

// Input sets the Input field of the Documentation
func (d *Documentation) Input(x string) {
	d.input = x
}

// Output sets the Output field of the Documentation
func (d *Documentation) Output(x string) {
	d.output = x
}

// AddMapper adds a new Mapper value to the mappers in Documentation
func (d *Documentation) AddMapper(input, output, description string) {
	d.mappers = append(d.mappers, Mapper{
		Input:       input,
		Output:      output,
		Description: description,
	})
}

func applyOpts(field *FieldDocs, opts []docOption) {
	for _, o := range opts {
		switch v := o.(type) {
		case SummaryString:
			field.Summary = string(v)
		case Default:
			field.Default = string(v)
		case EnvVar:
			field.EnvVar = string(v)
		case *SubFieldDoc:
			if len(field.discoveredFields) > 0 {
				v.merge(field.discoveredFields)
			}

			field.SubFields = v.Fields()
			field.Category = true
		}
	}
}

type SubFieldDoc struct {
	fields map[string]*FieldDocs
}

func (s *SubFieldDoc) docOption() bool { return true }

func (s *SubFieldDoc) merge(m map[string]*FieldDocs) {
	for k, f := range m {
		if e, ok := s.fields[k]; ok {
			e.Type = f.Type
			e.Optional = f.Optional
		} else {
			s.fields[k] = f
		}
	}
}

func (s *SubFieldDoc) SetField(name, synposis string, opts ...docOption) error {
	field, ok := s.fields[name]
	if !ok {
		field = &FieldDocs{
			Field:    name,
			Synopsis: synposis,
		}
		s.fields[name] = field
	} else {
		field.Synopsis = synposis
	}

	applyOpts(field, opts)

	return nil
}

// Fields returns the formatted FieldDocs values for the fields
func (d *SubFieldDoc) Fields() []*FieldDocs {
	var keys []string

	for k := range d.fields {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var fields []*FieldDocs

	for _, k := range keys {
		fields = append(fields, d.fields[k])
	}

	return fields
}

func SubFields(f func(d *SubFieldDoc)) *SubFieldDoc {
	sf := &SubFieldDoc{
		fields: make(map[string]*FieldDocs),
	}

	f(sf)

	return sf
}

// SetField sets various documentation for the given field. If the field is already
// known, the documentation is mearly updated. If the field is missing, it is created.
func (d *Documentation) SetField(name, synposis string, opts ...docOption) error {
	field, ok := d.fields[name]
	if !ok {
		field = &FieldDocs{
			Field:    name,
			Synopsis: synposis,
		}
		d.fields[name] = field
	} else {
		field.Synopsis = synposis
	}

	applyOpts(field, opts)

	return nil
}

// SetTemplateField sets various documentation for the given template field.
// If the field is already known, the documentation is mearly updated.
// If the field is missing, it is created.
func (d *Documentation) SetTemplateField(name, synposis string, opts ...docOption) error {
	field, ok := d.templateFields[name]
	if !ok {
		field = &FieldDocs{
			Field:    name,
			Synopsis: synposis,
		}
		d.templateFields[name] = field
	} else {
		field.Synopsis = synposis
	}

	applyOpts(field, opts)

	return nil
}

// SetRequestField sets various documentation for the given request field.
// If the field is already known, the documentation is mearly updated.
// If the field is missing, it is created.
func (d *Documentation) SetRequestField(name, synposis string, opts ...docOption) error {
	field, ok := d.requestFields[name]
	if !ok {
		field = &FieldDocs{
			Field:    name,
			Synopsis: synposis,
		}
		d.requestFields[name] = field
	} else {
		field.Synopsis = synposis
	}

	applyOpts(field, opts)

	return nil
}

// OverrideField sets the documentation for the given field directly.
func (d *Documentation) OverrideField(f *FieldDocs) error {
	d.fields[f.Field] = f
	return nil
}

// OverrideField sets the documentation for the given template field directly.
func (d *Documentation) OverrideTemplateField(f *FieldDocs) error {
	d.templateFields[f.Field] = f
	return nil
}

// OverrideField sets the documentation for the given request field directly.
func (d *Documentation) OverrideRequestField(f *FieldDocs) error {
	d.requestFields[f.Field] = f
	return nil
}

// Details returns the formatted Details value from Documentation
func (d *Documentation) Details() *Details {
	return &Details{
		Example:     d.example,
		Description: d.description,
		Input:       d.input,
		Output:      d.output,
		Mappers:     d.mappers,
	}
}

// Fields returns the formatted FieldDocs values for the fields
func (d *Documentation) Fields() []*FieldDocs {
	var keys []string

	for k := range d.fields {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var fields []*FieldDocs

	for _, k := range keys {
		fields = append(fields, d.fields[k])
	}

	return fields
}

// Fields returns the formatted FieldDocs values for the template fields
func (d *Documentation) TemplateFields() []*FieldDocs {
	var keys []string
	for k := range d.templateFields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var fields []*FieldDocs
	for _, k := range keys {
		fields = append(fields, d.templateFields[k])
	}

	return fields
}

// Fields returns the formatted FieldDocs values for the request fields
func (d *Documentation) RequestFields() []*FieldDocs {
	var keys []string
	for k := range d.requestFields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var fields []*FieldDocs
	for _, k := range keys {
		fields = append(fields, d.requestFields[k])
	}

	return fields
}
