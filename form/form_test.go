package form

import (
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type User struct {
	Name     string       `form:"name" validate:"required"`
	Age      int          `form:"age"`
	Email    string       `form:"email"`
	IsActive bool         `form:"active"`
	Ignored  string       `form:"-"`      // Ignored field
	Custom   MyCustomType `form:"custom"` // Custom type with TextUnmarshaler
}

type MyCustomType struct {
	Value string
}

func (m *MyCustomType) UnmarshalText(text []byte) error {
	m.Value = string(text)
	return nil
}

func TestParseForm(t *testing.T) {
	tests := []struct {
		name    string
		values  url.Values
		want    User
		wantErr bool
	}{
		{
			name: "valid input",
			values: url.Values{
				"name":   {"John Doe"},
				"age":    {"30"},
				"email":  {"john.doe@example.com"},
				"active": {"true"},
				"custom": {"customValue"},
			},
			want: User{
				Name:     "John Doe",
				Age:      30,
				Email:    "john.doe@example.com",
				IsActive: true,
				Custom:   MyCustomType{Value: "customValue"},
			},
			wantErr: false,
		},
		{
			name: "missing required field",
			values: url.Values{
				"age":   {"30"},
				"email": {"john.doe@example.com"},
			},
			want:    User{},
			wantErr: true,
		},
		{
			name: "invalid boolean",
			values: url.Values{
				"name":   {"test"},
				"active": {"invalid"},
			},
			want:    User{Name: "test"},
			wantErr: false,
		},
		{
			name: "invalid integer",
			values: url.Values{
				"name": {"test"},
				"age":  {"invalid"},
			},
			want:    User{Name: "test"},
			wantErr: true,
		},
		{
			name: "empty values, and non required fields",
			values: url.Values{
				"name": {"test"},
			},
			want:    User{Name: "test"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := User{}
			err := Decode(tt.values, &got)

			if (err != nil) != tt.wantErr {
				t.Errorf("decodeForm() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if diff := cmp.Diff(tt.want, got); !tt.wantErr && diff != "" {
				t.Error(diff)
			}
		})
	}
}
