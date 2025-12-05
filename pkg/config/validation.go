package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

var validate = validator.New()

func init() {
	// Prefer mapstructure names in error messages; fall back to field name.
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		if name := fld.Tag.Get("mapstructure"); name != "" {
			return name
		}
		return fld.Name
	})
}

type Validatable interface {
	Validate() error
}

func formatValidationError(errs validator.ValidationErrors) error {
	var messages []string

	for _, err := range errs {
		key := canonicalKey(err)
		source := sourceInfo(key)
		value := err.Value()

		switch err.Tag() {
		case "required":
			messages = append(messages, fmt.Sprintf("%s is required but not provided (%s)", key, source))
		case "url":
			messages = append(messages, fmt.Sprintf("%s must be a valid URL (%s)", key, source))
		case "min":
			if err.Kind() == reflect.Slice || err.Kind() == reflect.Array {
				messages = append(messages, fmt.Sprintf("%s must have at least %s items (got %d; %s)", key, err.Param(), reflect.ValueOf(value).Len(), source))
			} else {
				messages = append(messages, fmt.Sprintf("%s must be at least %s (got %v; %s)", key, err.Param(), value, source))
			}
		case "max":
			if err.Kind() == reflect.Slice || err.Kind() == reflect.Array {
				messages = append(messages, fmt.Sprintf("%s must have at most %s items (got %d; %s)", key, err.Param(), reflect.ValueOf(value).Len(), source))
			} else {
				messages = append(messages, fmt.Sprintf("%s must be at most %s (got %v; %s)", key, err.Param(), value, source))
			}
		case "dive":
			// This happens when validating slice elements
			if strings.Contains(key, "[") {
				messages = append(messages, fmt.Sprintf("%s contains invalid URL (%s)", key, source))
			}
		default:
			messages = append(messages, fmt.Sprintf("%s failed validation: %s (got %v; %s)", key, err.Tag(), value, source))
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
			return formatValidationError(validationErrors)
		}
		return fmt.Errorf("config validation failed: %w", err)
	}
	return nil
}

// canonicalKey normalizes a validation namespace to a viper-friendly dot key.
func canonicalKey(fe validator.FieldError) string {
	ns := fe.Namespace()
	if ns == "" {
		ns = fe.Field()
	}
	parts := strings.Split(ns, ".")
	if len(parts) > 1 {
		// Drop a leading type name (e.g. FullServerConfig) if it looks like CamelCase.
		if parts[0] != strings.ToLower(parts[0]) {
			parts = parts[1:]
		}
	}
	return strings.Join(parts, ".")
}

// sourceInfo reports possible sources for a viper key (env/config/flag/default).
func sourceInfo(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "sources: unknown"
	}

	var parts []string

	// Check env var
	envKey := envVarForKey(key)
	if val, ok := os.LookupEnv(envKey); ok {
		parts = append(parts, fmt.Sprintf("env %s=%s", envKey, val))
	}

	// Check config file
	if cfg := viper.ConfigFileUsed(); cfg != "" && viper.InConfig(key) {
		parts = append(parts, fmt.Sprintf("config (%s)", cfg))
	}

	// If not env or config, assume flag or default. We assume defaults are correct, so a bad value here likely came from a flag.
	if len(parts) == 0 {
		parts = append(parts, "flag")
	} else {
		parts = append(parts, "flag")
	}

	return "sources: " + strings.Join(parts, ", ")
}

// envVarForKey mirrors the default Viper env key building (dot -> _, upper, prefix).
func envVarForKey(key string) string {
	key = strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
	prefix := viper.GetEnvPrefix()
	if prefix != "" {
		return prefix + "_" + key
	}
	return key
}
