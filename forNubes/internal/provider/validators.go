package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// stringOneOfValidator validates that a string value is one of the allowed values.
type stringOneOfValidator struct {
	values []string
}

func (v stringOneOfValidator) Description(ctx context.Context) string {
	return fmt.Sprintf("value must be one of: %v", v.values)
}

func (v stringOneOfValidator) MarkdownDescription(ctx context.Context) string {
	return fmt.Sprintf("value must be one of: %v", v.values)
}

func (v stringOneOfValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	for _, allowed := range v.values {
		if value == allowed {
			return
		}
	}

	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Invalid value",
		fmt.Sprintf("Value must be one of %v, got: %s", v.values, value),
	)
}

func StringOneOf(values ...string) validator.String {
	return stringOneOfValidator{
		values: values,
	}
}

// validJSONArrayValidator проверяет, что строка является валидным JSON массивом
type validJSONArrayValidator struct{}

func (v validJSONArrayValidator) Description(ctx context.Context) string {
	return "value must be a valid JSON array (e.g., [\"item1\", \"item2\"])"
}

func (v validJSONArrayValidator) MarkdownDescription(ctx context.Context) string {
	return "value must be a valid JSON array (e.g., `[\"item1\", \"item2\"]`)"
}

func (v validJSONArrayValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	if value == "" {
		// Пустая строка не валидна - должен быть либо null, либо JSON массив
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid JSON Array",
			"Empty string is not a valid JSON array. Use jsonencode([...]) or omit the attribute.",
		)
		return
	}

	var arr []interface{}
	if err := json.Unmarshal([]byte(value), &arr); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid JSON Array",
			fmt.Sprintf("Value must be a valid JSON array: %s", err),
		)
	}
}

func ValidJSONArray() validator.String {
	return validJSONArrayValidator{}
}

// validJSONArrayOrEmptyValidator проверяет JSON массив или разрешает пустую строку
// Используется для accessIpList где "" означает "доступ отовсюду"
type validJSONArrayOrEmptyValidator struct{}

func (v validJSONArrayOrEmptyValidator) Description(ctx context.Context) string {
	return "value must be a valid JSON array or empty string (empty = access from anywhere)"
}

func (v validJSONArrayOrEmptyValidator) MarkdownDescription(ctx context.Context) string {
	return "value must be a valid JSON array or empty string (empty = access from anywhere)"
}

func (v validJSONArrayOrEmptyValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	if value == "" {
		// Пустая строка разрешена - означает доступ отовсюду
		return
	}

	var arr []interface{}
	if err := json.Unmarshal([]byte(value), &arr); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid JSON Array",
			fmt.Sprintf("Value must be a valid JSON array or empty string: %s", err),
		)
	}
}

func ValidJSONArrayOrEmpty() validator.String {
	return validJSONArrayOrEmptyValidator{}
}
