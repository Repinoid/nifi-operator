package resources_core

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func FormatBool(v types.Bool) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	if v.ValueBool() {
		return "true"
	}
	return "false"
}

func FormatInt64(v types.Int64) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return fmt.Sprintf("%d", v.ValueInt64())
}

func FormatString(v types.String) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return v.ValueString()
}
