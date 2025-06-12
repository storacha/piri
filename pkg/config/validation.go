package config

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

type Validatable interface {
	Validate() error
}

// getFieldFlag extracts the flag name from struct tags for a given field name
func getFieldFlag(structType reflect.Type, fieldName string) string {
	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}

	field, found := structType.FieldByName(fieldName)
	if !found {
		return "--" + strings.ToLower(fieldName)
	}

	// Check if this field has a flag tag
	if flagTag := field.Tag.Get("flag"); flagTag != "" {
		return "--" + flagTag
	}

	// Default to lowercase field name
	return "--" + strings.ToLower(fieldName)
}


func formatValidationError(structType reflect.Type, errs validator.ValidationErrors) error {
	var messages []string

	for _, err := range errs {
		field := err.Field()
		
		// Get flag name from struct tags
		flag := getFieldFlag(structType, field)
		hint := fmt.Sprintf(" (see %s flag for help)", flag)

		switch err.Tag() {
		case "required":
			messages = append(messages, fmt.Sprintf("%s is required but not provided%s", field, hint))
		case "url":
			messages = append(messages, fmt.Sprintf("%s must be a valid URL%s", field, hint))
		case "min":
			if err.Type().Kind() == reflect.Slice {
				messages = append(messages, fmt.Sprintf("%s must have at least %s items%s", field, err.Param(), hint))
			} else if err.Type().Kind() == reflect.Uint {
				messages = append(messages, fmt.Sprintf("%s must be at least %s%s", field, err.Param(), hint))
			}
		case "max":
			messages = append(messages, fmt.Sprintf("%s must be at most %s%s", field, err.Param(), hint))
		case "dive":
			// This happens when validating slice elements
			if strings.Contains(field, "[") {
				messages = append(messages, fmt.Sprintf("%s contains invalid URL %s", field, hint))
			}
		default:
			messages = append(messages, fmt.Sprintf("%s failed validation: %s%s", field, err.Tag(), hint))
		}
	}

	if len(messages) == 1 {
		return fmt.Errorf("config validation error: %s", messages[0])
	}
	return fmt.Errorf("config validation errors:\n  - %s", strings.Join(messages, "\n  - "))
}

func validateConfig[T Validatable](cfg T) error {
	if err := validate.Struct(cfg); err != nil {
		// Convert validation errors to custom messages
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			return formatValidationError(reflect.TypeOf(cfg), validationErrors)
		}
		return fmt.Errorf("config validation failed: %w", err)
	}
	return nil
}
