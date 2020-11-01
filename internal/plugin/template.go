package plugin

import (
	"encoding/json"
	"reflect"

	"github.com/iancoleman/strcase"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
)

// templateData returns the template data for a result object. If v
// implements component.Template that value is used. Otherwise, we automatically
// infer the fields based on the exported fields of the struct.
func templateData(v interface{}) ([]byte, error) {
	// Determine our data
	var data map[string]interface{}
	if tpl, ok := v.(component.Template); ok {
		data = tpl.TemplateData()
	} else {
		data = templateDataFromConfig(v)
	}

	// If empty we don't do anything
	if len(data) == 0 {
		return nil, nil
	}

	// Encode as JSON
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, status.Errorf(codes.Aborted,
			"failed to JSON encode result template data: %s", err)
	}

	return encoded, nil
}

func templateDataFromConfig(v interface{}) map[string]interface{} {
	var result map[string]interface{}
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result: &result,
		DecodeHook: func(srcT, dstT reflect.Type, raw interface{}) (interface{}, error) {
			for srcT.Kind() == reflect.Ptr {
				srcT = srcT.Elem()
			}
			if srcT.Kind() != reflect.Struct {
				return raw, nil
			}

			val := reflect.ValueOf(raw)
			for val.Kind() == reflect.Ptr {
				val = val.Elem()
			}

			m := map[string]interface{}{}
			for i := 0; i < srcT.NumField(); i++ {
				sf := srcT.Field(i)
				if sf.PkgPath != "" {
					// ignore unexported fields
					continue
				}

				name := strcase.ToSnake(sf.Name)
				m[name] = val.Field(i).Interface()
			}

			return m, nil
		},
	})
	if err != nil {
		panic(err)
	}
	if err := dec.Decode(v); err != nil {
		panic(err)
	}

	return result
}
