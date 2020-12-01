package docs

import (
	"reflect"
	"strings"

	"github.com/iancoleman/strcase"
)

// FromFunc fills in the documentation it can from the main function
// for the component: DeployFunc, ReleaseFunc, etc. v can be nil and this
// will do nothing.
//
// This currently extracts:
//
//   * Template fields from the result type if the result type is
//     a concrete type (not an interface value).
//
func FromFunc(v interface{}) Option {
	return func(d *Documentation) error {
		v := reflect.ValueOf(v)
		if !v.IsValid() || v.Kind() != reflect.Func {
			return nil
		}

		t := v.Type()
		if err := funcExtractTemplateFields(d, t); err != nil {
			return err
		}

		return nil
	}
}

// funcExtractTemplateFields extracts the templateFields values from
// the given type. The type must be a struct (after all pointers are removed).
// If the struct implements component.Template, we'll use that directly,
// otherwise the fields will be inferred based on exported struct fields.
func funcExtractTemplateFields(d *Documentation, t reflect.Type) error {
	// If we have no return values we can't calculate template fields.
	if t.NumOut() <= 0 {
		return nil
	}

	// Initialize our fields, even if we error later.
	if d.templateFields == nil {
		d.templateFields = map[string]*FieldDocs{}
	}

	// We get the first output type and use that. It is either a concrete
	// type or an error. If it is an error we'll catch it below.
	out := t.Out(0)
	for {
		if out.Implements(templateType) {
			return funcExtractTemplateFieldsFromImpl(d, out)
		}

		if out.Kind() == reflect.Ptr {
			out = out.Elem()
			continue
		}

		break
	}

	// We can only detect template fields on a struct.
	if out.Kind() != reflect.Struct {
		return nil
	}

	// Go through each field and document it.
	for i := 0; i < out.NumField(); i++ {
		f := out.Field(i)
		if f.PkgPath != "" {
			// Ignore unexported
			continue
		}

		if strings.HasPrefix(f.Name, "XXX_") {
			// ignore proto internals
			continue
		}

		// Our fields from a struct are snake case.
		name := strcase.ToSnake(f.Name)

		d.templateFields[name] = &FieldDocs{
			Field: name,
			Type:  f.Type.String(),
		}
	}

	return nil
}

// funcExtractTemplateFieldsFromImpl extracts the template fields from a
// type that implements templateInterface.
func funcExtractTemplateFieldsFromImpl(d *Documentation, t reflect.Type) error {
	// We need to create a new instance of t
	out := reflect.New(t).Elem().MethodByName("TemplateData").Call([]reflect.Value{})
	fields := out[0].Interface().(map[string]interface{})

	// Go through each field and document it
	for k, v := range fields {
		d.templateFields[k] = &FieldDocs{
			Field: k,
			Type:  reflect.TypeOf(v).String(),
		}
	}

	return nil
}

// templateType is the type implemented by results that support
// template data.
var templateType = reflect.TypeOf((*interface {
	TemplateData() map[string]interface{}
})(nil)).Elem()
