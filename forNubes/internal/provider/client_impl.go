package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Helper struct for parameters
type InstanceParam struct {
	SvcOperationCfsParamId int
	ParamValue             string
}

type InstanceOperationRequest struct {
	Action string      `json:"action"`
	Params interface{} `json:"params"`
}

// Implement methods for NubesClient defined in provider.go

func (c *NubesClient) GetOperationId(ctx context.Context, serviceId int, opName string) (int, error) {
	// Logic to fetch operation ID if needed.
	// Based on HAR, we might not strictly need this if we pass "operation": "create"
	// But let's assume we return a dummy or look it up.
	// For now, return 0 as placeholder or implement lookup if API supports it.
	return 0, nil
}

func (c *NubesClient) CreateInstance(ctx context.Context, displayName string, serviceId int, svcOperationId int, params []InstanceParam) (string, string, error) {
	// 1. Create Instance Placeholder
	// Payload based on pg_admin.har: {"serviceId":96,"displayName":"...","descr":""}

	payload := map[string]interface{}{
		"serviceId":   serviceId,
		"displayName": displayName,
		"descr":       "",
	}

	instanceUid, err := c.postInstance(ctx, payload)
	if err != nil {
		return "", "", fmt.Errorf("failed to create instance placeholder: %w", err)
	}
	tflog.Info(ctx, fmt.Sprintf("Created Instance Placeholder: %s", instanceUid))

	// 2. Create Operation
	opPayload := map[string]interface{}{
		"instanceUid": instanceUid,
		"operation":   "create",
	}
	// If svcOperationId is valid (>0), maybe we use it? HAR just said "operation":"create".
	// We'll stick to "operation":"create".

	opUid, err := c.postIgnoreResponse(ctx, "/instanceOperations", opPayload, true)
	if err != nil {
		return "", "", fmt.Errorf("failed to create operation: %w", err)
	}
	tflog.Info(ctx, fmt.Sprintf("Created Operation: %s", opUid))

	// 3. Get operation details with parameters
	opPath := fmt.Sprintf("/instanceOperations/%s?fields=cfsParams", opUid)
	var opDetails GetOperationResponse
	if err := c.get(ctx, opPath, &opDetails); err != nil {
		return "", "", fmt.Errorf("failed to get operation details: %w", err)
	}

	// 3a. Submit parameters
	// Strategy:
	// 1. Send all explicitly provided 'params' (from Terraform config/resource)
	// 2. Iterate server-provided defaults (opDetails) for anything we missed and send defaults

	sentParams := make(map[int]bool)

	// Phase 1: Send explicit overrides
	for _, p := range params {
		valToSend := p.ParamValue
		// Simple normalization for empty values if needed
		if valToSend == "" {
			// Some fields might reject empty string? For now send as is
			// or apply the map/list fix if we knew the type.
			// But for explicit params, we assume caller knows best.
		}

		paramPayload := map[string]interface{}{
			"instanceOperationUid":   opUid,
			"svcOperationCfsParamId": p.SvcOperationCfsParamId,
			"paramValue":             valToSend,
		}

		_, err := c.postIgnoreResponse(ctx, "/instanceOperationCfsParams", paramPayload, false)
		if err != nil {
			return "", "", fmt.Errorf("failed to submit explicit param %d: %w", p.SvcOperationCfsParamId, err)
		}
		sentParams[p.SvcOperationCfsParamId] = true
	}

	// Phase 2: Fill in defaults from Server Metadata (if not already sent)
	for _, param := range opDetails.InstanceOperation.CfsParams {
		if _, sent := sentParams[param.SvcOperationCfsParamId]; sent {
			continue // Already sent in Phase 1
		}

		valToSend := ""
		if param.ParamValue != nil {
			valToSend = *param.ParamValue
		} else if param.DefaultValue != nil {
			valToSend = *param.DefaultValue
		}

		// Fix specific data type formatting (from tubulus example)
		if valToSend == "" {
			if param.DataType == "map" || param.DataType == "json" {
				valToSend = "{}"
			} else if param.DataType == "array" || param.DataType == "list" {
				valToSend = "[]"
			}
		}

		paramPayload := map[string]interface{}{
			"instanceOperationUid":   opUid,
			"svcOperationCfsParamId": param.SvcOperationCfsParamId,
			"paramValue":             valToSend,
		}
		_, err := c.postIgnoreResponse(ctx, "/instanceOperationCfsParams", paramPayload, false)
		if err != nil {
			return "", "", fmt.Errorf("failed to submit default param %d: %w", param.SvcOperationCfsParamId, err)
		}
	}

	// 4. Validate (Optional, seen in S3 HAR)
	validateUrl := fmt.Sprintf("/instanceOperations/%s/validate-cfs", opUid)
	tflog.Info(ctx, fmt.Sprintf("Validating operation: %s", validateUrl))
	_ = c.get(ctx, validateUrl, nil)

	// 5. Run Operation
	// HAR: POST .../instanceOperations/{ids}/run
	runUrl := fmt.Sprintf("/instanceOperations/%s/run", opUid)
	tflog.Info(ctx, fmt.Sprintf("Running operation: %s", runUrl))
	_, err = c.postIgnoreResponse(ctx, runUrl, map[string]interface{}{}, false)
	if err != nil {
		return "", "", fmt.Errorf("failed to run operation: %w", err)
	}

	return instanceUid, opUid, nil
}

// RunInstanceOperation is a helper to just fill params and run an existing OP UID
func (c *NubesClient) RunInstanceOperation(ctx context.Context, opUid string, params []InstanceParam) (string, string, error) {
	// Add params
	for _, param := range params {
		valToSend := param.ParamValue
		// Simple normalization
		if valToSend == "" {
			valToSend = "pass" // Default fallback if needed, or empty
		}

		paramPayload := map[string]interface{}{
			"instanceOperationUid":   opUid,
			"svcOperationCfsParamId": param.SvcOperationCfsParamId,
			"paramValue":             valToSend,
		}
		_, err := c.postIgnoreResponse(ctx, "/instanceOperationCfsParams", paramPayload, false)
		if err != nil {
			return "", "", fmt.Errorf("failed to add param %d: %w", param.SvcOperationCfsParamId, err)
		}
	}

	// Validate
	validateUrl := fmt.Sprintf("/instanceOperations/%s/validate-cfs", opUid)
	_ = c.get(ctx, validateUrl, nil)

	// Run
	runUrl := fmt.Sprintf("/instanceOperations/%s/run", opUid)
	_, err := c.postIgnoreResponse(ctx, runUrl, map[string]interface{}{}, false)
	if err != nil {
		return "", "", fmt.Errorf("failed to run operation: %w", err)
	}

	return "", opUid, nil
}

func (c *NubesClient) WaitForInstanceStatus(ctx context.Context, instanceUid string, targetStatus string) error {

	timeout := time.After(15 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for instance %s to reach status %s", instanceUid, targetStatus)
		case <-ticker.C:
			inst, err := c.GetInstanceState(ctx, instanceUid)
			if err != nil {
				tflog.Warn(ctx, fmt.Sprintf("Error checking state for %s: %s", instanceUid, err))
				continue
			}

			state := inst.ExplainedStatus
			tflog.Info(ctx, fmt.Sprintf("Instance %s current status: %s (target: %s, InProgress: %v)", instanceUid, state, targetStatus, inst.OperationIsInProgress))

			if strings.EqualFold(state, targetStatus) {
				return nil
			}

			lowerState := strings.ToLower(state)
			if strings.Contains(lowerState, "error") || strings.Contains(lowerState, "failed") {
				return fmt.Errorf("instance %s in error state: %s", instanceUid, state)
			}

			if !inst.OperationIsInProgress && !inst.OperationIsPending {
				if strings.EqualFold(state, targetStatus) {
					return nil
				}
				return fmt.Errorf("operation finished but target status %s not reached (current: %s)", targetStatus, state)
			}
		}
	}
}

func (c *NubesClient) WaitForOperation(ctx context.Context, opUid string) error {
	timeout := time.After(15 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for operation %s", opUid)
		case <-ticker.C:
			op, err := c.GetInstanceOperation(ctx, opUid)
			if err != nil {
				tflog.Warn(ctx, fmt.Sprintf("Error checking operation %s: %s", opUid, err))
				continue
			}

			// Fail immediately if there is a fatal error log, regardless of other states
			if op.ErrorLog != nil && *op.ErrorLog != "" {
				return fmt.Errorf("operation failed (ErrorLog present): %s", *op.ErrorLog)
			}

			if !op.IsInProgress && !op.IsPending && op.DtFinish != nil && *op.DtFinish != "" {
				if op.IsSuccessful != nil && *op.IsSuccessful {
					return nil
				}
				errMsg := "operation failed (isSuccessful=false)"
				// ErrorLog check is redundant here but kept for completeness of the structure
				if op.ErrorLog != nil && *op.ErrorLog != "" {
					errMsg = *op.ErrorLog
				}
				return fmt.Errorf("%s", errMsg)
			}
		}
	}
}

func (c *NubesClient) get(ctx context.Context, path string, target interface{}) error {
	url := fmt.Sprintf("%s%s", c.ApiEndpoint, path)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	if c.ApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.ApiToken)
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s failed with status %d: %s", path, resp.StatusCode, string(body))
	}

	if target != nil {
		return json.NewDecoder(resp.Body).Decode(target)
	}
	return nil
}

func (c *NubesClient) WaitForInstanceReady(ctx context.Context, instanceUid string) error {
	// Set timeout
	timeout := time.After(15 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for instance %s", instanceUid)
		case <-ticker.C:
			// check status
			inst, err := c.GetInstanceState(ctx, instanceUid)
			if err != nil {
				tflog.Warn(ctx, fmt.Sprintf("Error checking state for %s: %s", instanceUid, err))
				continue
			}

			state := inst.ExplainedStatus
			tflog.Info(ctx, fmt.Sprintf("Instance %s status: %s (InProgress: %v)", instanceUid, state, inst.OperationIsInProgress))

			// Statuses from HAR: "running", "Active", "deployed", "Deployed"
			if strings.EqualFold(state, "Active") || strings.EqualFold(state, "Running") || strings.EqualFold(state, "Deployed") {
				return nil
			}

			// Detect failure patterns in explainedStatus
			lowerState := strings.ToLower(state)
			if strings.Contains(lowerState, "error") ||
				strings.Contains(lowerState, "failed") ||
				strings.Contains(lowerState, "не удалось") ||
				strings.Contains(lowerState, "не заполнена") {
				return fmt.Errorf("instance %s end status: %s", instanceUid, state)
			}

			// If operation finished but we didn't reach success state
			if !inst.OperationIsInProgress && !inst.OperationIsPending {
				// Second check for success, sometimes status updates slightly after op finishes
				if strings.EqualFold(state, "Active") || strings.EqualFold(state, "Running") || strings.EqualFold(state, "Deployed") {
					return nil
				}
				// If still not success, and not in a known transitioning state (like "Processing", "Creating")
				if !strings.EqualFold(state, "Creating") && !strings.EqualFold(state, "Processing") && state != "" {
					return fmt.Errorf("operation finished but instance %s is in unexpected state: %s", instanceUid, state)
				}
			}
		}
	}
}

func (c *NubesClient) DeleteInstance(ctx context.Context, instanceUid string, opId int) error {
	// Create delete operation
	opPayload := map[string]interface{}{
		"instanceUid": instanceUid,
		"operation":   "delete",
	}
	opUid, err := c.postIgnoreResponse(ctx, "/instanceOperations", opPayload, true)
	if err != nil {
		return err
	}

	// Run It
	runUrl := fmt.Sprintf("/instanceOperations/%s/run", opUid)
	_, err = c.postIgnoreResponse(ctx, runUrl, map[string]interface{}{}, false)
	return err
}

// Helpers

func (c *NubesClient) postInstance(ctx context.Context, payload interface{}) (string, error) {
	url := fmt.Sprintf("%s/instances", c.ApiEndpoint)
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.ApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.ApiToken)
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	// Try to get ID from Location header
	loc := resp.Header.Get("Location")
	if loc != "" {
		parts := strings.Split(loc, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	// Try body
	var res map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	if len(body) > 0 {
		if err := json.Unmarshal(body, &res); err == nil {
			if uid, ok := res["instanceUid"].(string); ok {
				return uid, nil
			}
		}
	}

	return "", fmt.Errorf("could not extract instanceUid from response")
}

func (c *NubesClient) postIgnoreResponse(ctx context.Context, path string, payload interface{}, returnLocationId bool) (string, error) {
	url := fmt.Sprintf("%s%s", c.ApiEndpoint, path)
	var reqBody io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return "", err
		}
		reqBody = bytes.NewBuffer(data)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, reqBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.ApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.ApiToken)
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	if returnLocationId {
		// Extract ID from Location header .../something/{id}
		loc := resp.Header.Get("Location")
		if loc != "" {
			parts := strings.Split(loc, "/")
			return parts[len(parts)-1], nil
		}
		// Fallback to body scan if needed
		var res map[string]interface{}
		body, _ := io.ReadAll(resp.Body)
		if len(body) > 0 {
			if err := json.Unmarshal(body, &res); err == nil {
				// Try common ID fields
				if id, ok := res["instanceOperationUid"].(string); ok {
					return id, nil
				}
			}
		}
	}
	return "", nil
}

type ApiOperation struct {
	SvcOperationId int    `json:"svcOperationId"`
	Operation      string `json:"operation"`
}

type InstanceStateData struct {
	Params map[string]interface{} `json:"params"`
}

type InstanceStateResponse struct {
	InstanceUid           string             `json:"instanceUid"`
	ExplainedStatus       string             `json:"explainedStatus"`
	OperationIsInProgress bool               `json:"operationIsInProgress"`
	OperationIsPending    bool               `json:"operationIsPending"`
	AvailableOperations   []ApiOperation     `json:"availableOperations"`
	State                 *InstanceStateData `json:"state"`
}

func (c *NubesClient) GetInstanceState(ctx context.Context, instanceUid string) (*InstanceStateResponse, error) {
	url := fmt.Sprintf("%s/instances/%s", c.ApiEndpoint, instanceUid)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if c.ApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.ApiToken)
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var res struct {
		Instance InstanceStateResponse `json:"instance"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return &res.Instance, nil
}

// Post is a helper for generic API calls, used by postgres_resource
func (c *NubesClient) Post(path string, payload interface{}, target interface{}) error {
	url := fmt.Sprintf("%s%s", c.ApiEndpoint, path)
	var reqBody io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		reqBody = bytes.NewBuffer(data)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.ApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.ApiToken)
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	if target != nil {
		return json.NewDecoder(resp.Body).Decode(target)
	}
	return nil
}

func (c *NubesClient) GetInstanceStateDetails(ctx context.Context, instanceUid string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/instances/%s", c.ApiEndpoint, instanceUid)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if c.ApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.ApiToken)
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var res map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if inst, ok := res["instance"].(map[string]interface{}); ok {
		if state, ok := inst["state"].(map[string]interface{}); ok {
			return state, nil
		}
	}
	return nil, fmt.Errorf("could not find instance.state in response")
}

func (c *NubesClient) GetOperationIdForInstance(ctx context.Context, instanceUid string, opName string) (int, error) {
	url := fmt.Sprintf("%s/instances/%s", c.ApiEndpoint, instanceUid)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	if c.ApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.ApiToken)
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}

	var res map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return 0, err
	}

	inst, ok := res["instance"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("instance field missing")
	}

	availableOps, ok := inst["availableOperations"].([]interface{})
	if !ok {
		return 0, fmt.Errorf("availableOperations missing or empty")
	}

	for _, item := range availableOps {
		opMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		var name string
		var id int

		if n, ok := opMap["name"].(string); ok {
			name = n
		}
		if n, ok := opMap["operation"].(string); ok {
			name = n
		}
		if i, ok := opMap["svcOperationId"].(float64); ok {
			id = int(i)
		}

		if svcOp, ok := opMap["svcOperation"].(map[string]interface{}); ok {
			if n, ok := svcOp["name"].(string); ok {
				name = n
			}
			if idFloat, ok := svcOp["id"].(float64); ok {
				id = int(idFloat)
			}
		}

		if strings.EqualFold(name, opName) {
			return id, nil
		}
	}

	return 0, fmt.Errorf("operation '%s' not found in available operations", opName)
}

type GetOperationResponse struct {
	InstanceOperation OperationResponse `json:"instanceOperation"`
}

// OperationResponse представляет собой полное состояние операции из Deck API.
// Все поля получены путем анализа HAR файлов (dummy.har, vdc.har) и документации платформы.
type OperationResponse struct {
	InstanceOperationUid string     `json:"instanceOperationUid"` // Уникальный идентификатор операции (UUID)
	InstanceUid          string     `json:"instanceUid"`          // Идентификатор инстанса, над которым идет работа
	Operation            string     `json:"operation"`            // Имя операции (create, modify, suspend, и т.д.)
	IsInProgress         bool       `json:"isInProgress"`         // true, если Jenkins-джоб выполняется в данный момент
	IsPending            bool       `json:"isPending"`            // true, если операция стоит в очереди (ждем слота в Jenkins)
	IsSuccessful         *bool      `json:"isSuccessful"`         // "Зеленая галка" платформы. Появляется ПОСЛЕ dtFinish.
	DtFinish             *string    `json:"dtFinish"`             // Штамп времени окончания (ГГГГ-ММ-ДД...). Не null = ОПЕРАЦИЯ ЗАВЕРШЕНА.
	DtCreated            *string    `json:"dtCreated"`            // Время создания записи об операции
	DtUpdated            *string    `json:"dtUpdated"`            // Время последнего обновления записи
	DtSubmit             *string    `json:"dtSubmit"`             // Когда кнопка была нажата (или API вызван)
	DtStart              *string    `json:"dtStart"`              // Когда реально начался Jenkins-джоб
	SubmitResult         *string    `json:"submitResult"`         // HTTP-код первичной регистрации (обычно "201")
	Duration             *float64   `json:"duration"`             // Общее время выполнения в секундах
	ErrorLog             *string    `json:"errorLog"`             // Текст ошибки, если операция упала
	UpdaterId            *int       `json:"updaterId"`            // ID пользователя, запустившего операцию
	UpdaterLogin         *string    `json:"updaterLogin"`         // Логин инициатора
	UpdaterShortname     *string    `json:"updaterShortname"`     // Инициалы инициатора (например, "Н. Ф.")
	DisplayName          *string    `json:"displayName"`          // Имя инстанса на момент операции
	ServiceId            *int       `json:"serviceId"`            // ID сервиса (1 - Болванка, 96 - PG Admin и т.д.)
	Svc                  *string    `json:"svc"`                  // Текстовое имя сервиса
	SvcOperationId       *int       `json:"svcOperationId"`       // Внутренний ID операции в каталоге
	Man                  *string    `json:"man"`                  // Мануал/описание операции (иногда содержит Markdown)
	CfsParams            []CfsParam `json:"cfsParams"`            // Список всех параметров (конфигурация)
	Stages               []ApiStage `json:"stages"`               // Этапы выполнения джоба (подготовка, секреты...)
	State                any        `json:"state"`                // Результирующее состояние (выходные данные джоба)
}

// ApiStage представляет этап выполнения операции в Jenkins
type ApiStage struct {
	InstanceOperationStageUid string  `json:"instanceOperationStageUid"`
	Stage                     string  `json:"stage"`        // Название (например, "Подготовка среды")
	IsSuccessful              bool    `json:"isSuccessful"` // Успех конкретного этапа
	DtStart                   *string `json:"dtStart"`
	DtFinish                  *string `json:"dtFinish"`
	Duration                  float64 `json:"duration"`
	StageMsg                  *string `json:"stageMsg"` // Лог этапа (часто JSON в строке)
}

type CfsParam struct {
	InstanceOperationCfsParamUid string  `json:"instanceOperationCfsParamUid"`
	SvcOperationCfsParamId       int     `json:"svcOperationCfsParamId"`
	ParamValue                   *string `json:"paramValue"`
	DefaultValue                 *string `json:"defaultValue"`
	DataType                     string  `json:"dataType"`
	Name                         string  `json:"name"`
	Code                         string  `json:"code"`
	SvcOperationCfsParam         string  `json:"svcOperationCfsParam"`
}

func (c *NubesClient) GetInstanceOperation(ctx context.Context, opUid string) (*OperationResponse, error) {
	// ВАЖНО: Список полей максимально расширен на основе HAR (dummy.har, vdc.har).
	// Эти поля позволяют видеть полную картину происходящего на платформе.
	// fields := "instanceOperationUid,instanceUid,state,stages,cfsParams,isSuccessful,dtCreated,dtUpdated,dtStart,dtFinish,operation,svcOperationId,svc,displayName,submitResult,duration,errorLog,updaterShortname,man,isInProgress,isPending,dtSubmit"
	// url := fmt.Sprintf("%s/instanceOperations/%s?fields=%s", c.ApiEndpoint, opUid, fields)
	// Try without fields to see if we get UIDs
	url := fmt.Sprintf("%s/instanceOperations/%s", c.ApiEndpoint, opUid)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if c.ApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.ApiToken)
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	tflog.Info(ctx, fmt.Sprintf("GetInstanceOperation RAW RESPONSE: %s", string(body)))

	var res GetOperationResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w, body: %s", err, string(body))
	}

	return &res.InstanceOperation, nil
}

func (c *NubesClient) RunAction(ctx context.Context, instanceUid string, action string) error {
	state, err := c.GetInstanceState(ctx, instanceUid)
	if err != nil {
		return err
	}

	var opId int
	for _, op := range state.AvailableOperations {
		if op.Operation == action {
			opId = op.SvcOperationId
			break
		}
	}

	if opId == 0 {
		return fmt.Errorf("action %s not available for instance %s", action, instanceUid)
	}

	// 1. Create Op
	payload := map[string]interface{}{
		"instanceUid":    instanceUid,
		"svcOperationId": opId,
		"operation":      action,
	}

	opUid, err := c.postIgnoreResponse(ctx, "/instanceOperations", payload, true)
	if err != nil {
		return fmt.Errorf("failed to create %s operation: %w", action, err)
	}
	if opUid == "" {
		return fmt.Errorf("failed to get operation UID for %s", action)
	}

	// 2. Run Op
	runUrl := fmt.Sprintf("/instanceOperations/%s/run", opUid)
	_, err = c.postIgnoreResponse(ctx, runUrl, map[string]interface{}{}, false)
	return err
}

type InstanceSummary struct {
	InstanceUid string `json:"instanceUid"`
	DisplayName string `json:"displayName"`
	ServiceId   int    `json:"serviceId"`
	Svc         string `json:"svc"`
}

type InstancesListResponse struct {
	Results []InstanceSummary `json:"results"`
}

func (c *NubesClient) GetInstances(ctx context.Context) ([]InstanceSummary, error) {
	var allInstances []InstanceSummary
	page := 1

	for {
		url := fmt.Sprintf("%s/instances?page=%d&size=100", c.ApiEndpoint, page)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		if c.ApiToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.ApiToken)
		}

		resp, err := c.HttpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("status %d", resp.StatusCode)
		}

		var res InstancesListResponse
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return nil, err
		}

		if len(res.Results) == 0 {
			break
		}

		allInstances = append(allInstances, res.Results...)
		page++

		// Safety break to prevent infinite loops if API behaves weirdly
		if page > 100 {
			break
		}
	}

	return allInstances, nil
}

func (c *NubesClient) GetInstanceFull(ctx context.Context, instanceUid string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/instances/%s", c.ApiEndpoint, instanceUid)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if c.ApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.ApiToken)
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var res map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if inst, ok := res["instance"].(map[string]interface{}); ok {
		return inst, nil
	}
	return nil, fmt.Errorf("instance field missing in response")
}
