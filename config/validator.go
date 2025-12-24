package config

import (
	"fmt"
	"reflect"
	"strings"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}

	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// StructValidator validates configuration using struct tags
type StructValidator struct {
	target interface{}
}

// NewStructValidator creates a validator for a struct
func NewStructValidator(target interface{}) *StructValidator {
	return &StructValidator{target: target}
}

// Validate validates the configuration against struct tags
// Supported tags:
//   - validate:"required" - field must be set
//   - validate:"min=X" - minimum value for numbers, minimum length for strings
//   - validate:"max=X" - maximum value for numbers, maximum length for strings
//   - validate:"oneof=A B C" - value must be one of the specified options
func (sv *StructValidator) Validate(config interface{}) error {
	// First unmarshal config into target struct
	if err := UnmarshalEnv(config.(map[string]interface{}), sv.target); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Then validate
	return sv.validateStruct(reflect.ValueOf(sv.target).Elem(), "")
}

// validateStruct recursively validates a struct
func (sv *StructValidator) validateStruct(v reflect.Value, prefix string) error {
	var errors ValidationErrors

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		fieldName := fieldType.Name
		if prefix != "" {
			fieldName = prefix + "." + fieldName
		}

		// Recurse into nested structs
		if field.Kind() == reflect.Struct {
			if err := sv.validateStruct(field, fieldName); err != nil {
				if verrs, ok := err.(ValidationErrors); ok {
					errors = append(errors, verrs...)
				}
			}
			continue
		}

		// Get validation tag
		validateTag := fieldType.Tag.Get("validate")
		if validateTag == "" {
			continue
		}

		// Parse validation rules
		rules := strings.Split(validateTag, ",")
		for _, rule := range rules {
			rule = strings.TrimSpace(rule)

			if err := sv.validateRule(field, fieldName, rule); err.Message != "" {
				errors = append(errors, err)
			}
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateRule validates a single rule
func (sv *StructValidator) validateRule(field reflect.Value, fieldName, rule string) ValidationError {
	parts := strings.SplitN(rule, "=", 2)
	ruleName := parts[0]
	ruleValue := ""
	if len(parts) == 2 {
		ruleValue = parts[1]
	}

	switch ruleName {
	case "required":
		if isZeroValue(field) {
			return ValidationError{
				Field:   fieldName,
				Message: "field is required",
			}
		}

	case "min":
		if err := sv.validateMin(field, fieldName, ruleValue); err.Message != "" {
			return err
		}

	case "max":
		if err := sv.validateMax(field, fieldName, ruleValue); err.Message != "" {
			return err
		}

	case "oneof":
		if err := sv.validateOneOf(field, fieldName, ruleValue); err.Message != "" {
			return err
		}
	}

	return ValidationError{}
}

// validateMin validates minimum value/length
func (sv *StructValidator) validateMin(field reflect.Value, fieldName, minStr string) ValidationError {
	switch field.Kind() {
	case reflect.String:
		var min int
		fmt.Sscanf(minStr, "%d", &min)
		if len(field.String()) < min {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("must be at least %d characters", min),
			}
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var min int64
		fmt.Sscanf(minStr, "%d", &min)
		if field.Int() < min {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("must be at least %d", min),
			}
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var min uint64
		fmt.Sscanf(minStr, "%d", &min)
		if field.Uint() < min {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("must be at least %d", min),
			}
		}
	}

	return ValidationError{}
}

// validateMax validates maximum value/length
func (sv *StructValidator) validateMax(field reflect.Value, fieldName, maxStr string) ValidationError {
	switch field.Kind() {
	case reflect.String:
		var max int
		fmt.Sscanf(maxStr, "%d", &max)
		if len(field.String()) > max {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("must be at most %d characters", max),
			}
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var max int64
		fmt.Sscanf(maxStr, "%d", &max)
		if field.Int() > max {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("must be at most %d", max),
			}
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var max uint64
		fmt.Sscanf(maxStr, "%d", &max)
		if field.Uint() > max {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("must be at most %d", max),
			}
		}
	}

	return ValidationError{}
}

// validateOneOf validates value is one of allowed options
func (sv *StructValidator) validateOneOf(field reflect.Value, fieldName, options string) ValidationError {
	allowed := strings.Fields(options)
	value := fmt.Sprintf("%v", field.Interface())

	for _, opt := range allowed {
		if value == opt {
			return ValidationError{}
		}
	}

	return ValidationError{
		Field:   fieldName,
		Message: fmt.Sprintf("must be one of: %s", strings.Join(allowed, ", ")),
	}
}

// isZeroValue checks if a field has its zero value
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	}
	return false
}

// FuncValidator wraps a validation function
type FuncValidator struct {
	fn func(interface{}) error
}

// NewFuncValidator creates a validator from a function
func NewFuncValidator(fn func(interface{}) error) *FuncValidator {
	return &FuncValidator{fn: fn}
}

// Validate executes the validation function
func (fv *FuncValidator) Validate(config interface{}) error {
	return fv.fn(config)
}

// ChainValidator chains multiple validators
type ChainValidator struct {
	validators []Validator
}

// NewChainValidator creates a validator that runs multiple validators in sequence
func NewChainValidator(validators ...Validator) *ChainValidator {
	return &ChainValidator{validators: validators}
}

// Validate runs all validators in the chain
func (cv *ChainValidator) Validate(config interface{}) error {
	var errors ValidationErrors

	for _, validator := range cv.validators {
		if err := validator.Validate(config); err != nil {
			if verrs, ok := err.(ValidationErrors); ok {
				errors = append(errors, verrs...)
			} else {
				errors = append(errors, ValidationError{
					Field:   "unknown",
					Message: err.Error(),
				})
			}
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}
