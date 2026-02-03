// ВНИМАНИЕ: НЕ ИЗМЕНЯТЬ НИЧЕГО В CRUD БЕЗ ПРЯМОГО РАЗРЕШЕНИЯ ОПЕРАТОРА.
package resources_gen

import (
	"context"
	"fmt"
	"strings"

	"terraform-provider-nubes/internal/core"
)

// CreateResource uses the universal client flow for create/resume/adopt.
func CreateResource(ctx context.Context, client *core.UniversalClient, serviceID int, displayName string, resumeIfExists bool, params map[int]string) (string, error) {
	existing, err := client.FindInstanceByDisplayName(ctx, serviceID, displayName)
	if err != nil {
		return "", err
	}
	if existing != nil {
		if !resumeIfExists {
			return "", fmt.Errorf("resource with resource_name already exists: %s", displayName)
		}
		status := strings.ToLower(existing.ExplainedStatus)
		if strings.Contains(status, "suspend") || strings.Contains(status, "suspended") {
			if err := client.RunInstanceOperationUniversal(ctx, existing.InstanceUid, "resume", nil); err != nil {
				return "", err
			}
		}
		return existing.InstanceUid, nil
	}

	return client.CreateGenericInstanceUniversalV6(ctx, serviceID, displayName, params)
}

// UpdateResource uses universal modify flow with defaults.
func UpdateResource(ctx context.Context, client *core.UniversalClient, instanceID string, params map[int]string) error {
	if strings.TrimSpace(instanceID) == "" {
		return fmt.Errorf("missing instance id for modify")
	}
	return client.RunInstanceOperationUniversalWithDefaults(ctx, instanceID, "modify", params)
}

// DeleteResource runs delete/suspend or removes from state.
func DeleteResource(ctx context.Context, client *core.UniversalClient, instanceID string, deleteMode string) error {
	mode := strings.ToLower(strings.TrimSpace(deleteMode))
	if mode == "" {
		mode = "state_only"
	}

	switch mode {
	case "state_only":
		return nil
	case "suspend", "delete":
		return client.RunInstanceOperationUniversal(ctx, instanceID, mode, nil)
	default:
		return fmt.Errorf("invalid delete_mode: %s", deleteMode)
	}
}
