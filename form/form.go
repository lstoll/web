package form

import (
	"encoding"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// ErrInvalidForm is returned when a form does not contain required fields, or
// other validations fail
type ErrInvalidForm struct {
	MissingFields []string
}

func (e *ErrInvalidForm) Error() string {
	return fmt.Sprintf("Form is missing the following fields: %s", strings.Join(e.MissingFields, ", "))
}

// Decode parses a url.Values into a struct based on struct tags.
func Decode(values url.Values, into any) error {
	v := reflect.ValueOf(into).Elem()
	t := v.Type()

	var errInvalidForm *ErrInvalidForm

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		formTag := field.Tag.Get("form")
		validateTag := field.Tag.Get("validate")

		if formTag == "-" { // Ignore field
			continue
		}

		if formTag == "" {
			continue // Skip fields without form tag
		}

		formValue := values.Get(formTag)

		if validateTag == "required" && formValue == "" {
			if errInvalidForm == nil {
				errInvalidForm = &ErrInvalidForm{}
			}
			errInvalidForm.MissingFields = append(errInvalidForm.MissingFields, field.Name)
			continue
		}

		if formValue == "" {
			continue // Skip empty values if not required
		}

		fieldValue := v.Field(i)

		// Check for TextUnmarshaler interface
		if unmarshaler, ok := fieldValue.Addr().Interface().(encoding.TextUnmarshaler); ok {
			err := unmarshaler.UnmarshalText([]byte(formValue))
			if err != nil {
				return fmt.Errorf("error unmarshaling field '%s': %v", field.Name, err)
			}
			continue // Move to the next field
		}

		switch fieldValue.Kind() {
		case reflect.String:
			fieldValue.SetString(formValue)
		case reflect.Int:
			intValue, err := strconv.Atoi(formValue)
			if err != nil {
				return fmt.Errorf("invalid integer value for field '%s': %v", field.Name, err)
			}
			fieldValue.SetInt(int64(intValue))
		case reflect.Bool:
			boolValue := strings.ToLower(formValue) == "true" || formValue == "1" || strings.ToLower(formValue) == "on"
			fieldValue.SetBool(boolValue)
		// Add more cases for other types as needed
		default:
			return fmt.Errorf("unsupported field type for field '%s'", field.Name)
		}
	}

	if errInvalidForm != nil {
		return errInvalidForm
	}
	return nil
}
