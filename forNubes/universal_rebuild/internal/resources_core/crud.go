// ВНИМАНИЕ: НЕ ИЗМЕНЯТЬ НИЧЕГО В CRUD БЕЗ ПРЯМОГО РАЗРЕШЕНИЯ ОПЕРАТОРА.
package resources_core

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
		status := strings.ToLower(strings.TrimSpace(existing.ExplainedStatus))
		if isStatusNonAdoptable(status) {
			return "", fmt.Errorf("resource exists but not ready for adopt: %s", existing.ExplainedStatus)
		}
		if isStatusSuspended(status) {
			if err := client.RunInstanceOperationUniversal(ctx, existing.InstanceUid, "resume", nil); err != nil {
				return "", err
			}
			resumed, err := client.GetInstanceState(ctx, existing.InstanceUid)
			if err != nil {
				return "", err
			}
			resumedStatus := strings.ToLower(strings.TrimSpace(resumed.ExplainedStatus))
			if isStatusNonAdoptable(resumedStatus) || isStatusSuspended(resumedStatus) {
				return "", fmt.Errorf("resource not ready after resume: %s", resumed.ExplainedStatus)
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

func isStatusSuspended(status string) bool {
	return strings.Contains(status, "suspend") || strings.Contains(status, "suspended")
}

func isStatusNonAdoptable(status string) bool {
	if status == "" {
		return true
	}
	if strings.Contains(status, "not created") {
		return true
	}
	if strings.Contains(status, "pending") || strings.Contains(status, "creating") {
		return true
	}
	if strings.Contains(status, "failed") || strings.Contains(status, "error") {
		return true
	}
	return false
}
