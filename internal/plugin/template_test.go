package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTemplateDataFromConfig(t *testing.T) {
	cases := []struct {
		Name   string
		Input  interface{}
		Output map[string]interface{}
	}{
		{
			"all-in-one",
			struct {
				Name      string
				Port      int
				ImageName string
			}{
				Name:      "Hello",
				Port:      80,
				ImageName: "foo",
			},
			map[string]interface{}{
				"name":       "Hello",
				"port":       80,
				"image_name": "foo",
			},
		},

		{
			"pointer",
			&struct {
				Name string
				Port int
			}{
				Name: "Hello",
				Port: 80,
			},
			map[string]interface{}{
				"name": "Hello",
				"port": 80,
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {
			require := require.New(t)

			v := templateDataFromConfig(tt.Input)
			require.Equal(tt.Output, v)
		})
	}
}
