package docs

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"
)

func TestFromFunc(t *testing.T) {
	cases := []struct {
		Name     string
		Input    interface{}
		Expected []*FieldDocs
		Err      string
	}{
		{
			"pointer to a struct",
			func() (*struct {
				Image    string
				FullName string
			}, error) {
				return nil, nil
			},
			[]*FieldDocs{
				&FieldDocs{
					Field: "full_name",
					Type:  "string",
				},
				&FieldDocs{
					Field: "image",
					Type:  "string",
				},
			},
			"",
		},

		{
			"struct implementing interface",
			func() (*testTemplateStruct, error) {
				return nil, nil
			},
			[]*FieldDocs{
				&FieldDocs{
					Field: "full_name",
					Type:  "string",
				},
				&FieldDocs{
					Field: "image",
					Type:  "string",
				},
			},
			"",
		},
	}

	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {
			require := require.New(t)

			d, err := New(FromFunc(tt.Input))
			if tt.Err != "" {
				require.Error(err)
				require.Contains(err.Error(), tt.Err)
				return
			}
			require.NoError(err)

			require.Equal(tt.Expected, d.TemplateFields(), spew.Sdump(d.TemplateFields()))
		})
	}
}

type testTemplateStruct struct{}

func (t *testTemplateStruct) TemplateData() map[string]interface{} {
	return map[string]interface{}{
		"image":     "",
		"full_name": "",
	}
}
