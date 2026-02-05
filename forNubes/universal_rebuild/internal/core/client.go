// ВНИМАНИЕ: НЕ ИЗМЕНЯТЬ НИЧЕГО В ЯДРЕ БЕЗ ПРЯМОГО РАЗРЕШЕНИЯ ОПЕРАТОРА.
package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// UniversalClient handles Nubes API logic.
type UniversalClient struct {
	HttpClient  *http.Client
	ApiEndpoint string
	ApiToken    string
}

type genericInstanceReq struct {
	ServiceId   int    `json:"serviceId"`
	DisplayName string `json:"displayName"`
	Descr       string `json:"descr"`
}

type genericOpReq struct {
	InstanceUid string `json:"instanceUid"`
	Operation   string `json:"operation"`
}

type genericParamReq struct {
	InstanceOperationUid   string `json:"instanceOperationUid"`
	SvcOperationCfsParamId int    `json:"svcOperationCfsParamId"`
	ParamValue             string `json:"paramValue"`
}

// ===== UNIVERSAL FLOW V6 (baseline) =====

type universalOpResponse struct {
	InstanceOperation universalOperation `json:"instanceOperation"`
}

type universalOperation struct {
	CfsParams []universalCfsParam `json:"cfsParams"`
}

type universalCfsParam struct {
	SvcOperationCfsParamId int     `json:"svcOperationCfsParamId"`
	ParamValue             *string `json:"paramValue"`
	DefaultValue           *string `json:"defaultValue"`
	IsRequired             bool    `json:"isRequired"`
	DataType               string  `json:"dataType"`
	Name                   string  `json:"name"`
	Code                   string  `json:"code"`
	Label                  string  `json:"label"`
	SvcOperationCfsParam   string  `json:"svcOperationCfsParam"`
	RefSvcId               *int    `json:"refSvcId"`
}

// CreateGenericInstanceUniversalV6 implements the universal flow:
// instances -> instanceOperations -> get cfsParams -> submit params -> validate -> run
func (c *UniversalClient) CreateGenericInstanceUniversalV6(ctx context.Context, serviceId int, displayName string, params map[int]string) (string, error) {
	instPayload := genericInstanceReq{
		ServiceId:   serviceId,
		DisplayName: displayName,
		Descr:       "Created via Terraform Universal Provider",
	}

	instResp, instHeaders, err := c.doRequest(ctx, "POST", "/instances", instPayload)
	if err != nil {
		return "", err
	}

	instanceUid := extractUIDFromLocation(instHeaders.Get("Location"))
	if instanceUid == "" {
		var instResult struct {
			InstanceUid string `json:"instanceUid"`
		}
		if err := json.Unmarshal(instResp, &instResult); err == nil && instResult.InstanceUid != "" {
			instanceUid = instResult.InstanceUid
		} else {
			var justId string
			if err2 := json.Unmarshal(instResp, &justId); err2 == nil && justId != "" {
				instanceUid = justId
			}
		}
	}
	if instanceUid == "" {
		return "", fmt.Errorf("could not extract instanceUid from response (Header: %s)", instHeaders.Get("Location"))
	}

	opPayload := genericOpReq{
		InstanceUid: instanceUid,
		Operation:   "create",
	}

	opResp, opHeaders, err := c.doRequest(ctx, "POST", "/instanceOperations", opPayload)
	if err != nil {
		return "", err
	}

	opUid := extractUIDFromLocation(opHeaders.Get("Location"))
	if opUid == "" {
		var opResult struct {
			InstanceOperationUid string `json:"instanceOperationUid"`
		}
		if err := json.Unmarshal(opResp, &opResult); err == nil && opResult.InstanceOperationUid != "" {
			opUid = opResult.InstanceOperationUid
		} else {
			var justId string
			if err2 := json.Unmarshal(opResp, &justId); err2 == nil && justId != "" {
				opUid = justId
			}
		}
	}
	if opUid == "" {
		return "", fmt.Errorf("failed to extract instanceOperationUid (Header: %s)", opHeaders.Get("Location"))
	}

	opDetailsResp, _, err := c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s?fields=cfsParams", opUid), nil)
	if err != nil {
		return "", fmt.Errorf("failed to get operation details: %w", err)
	}
	var opDetails universalOpResponse
	if err := json.Unmarshal(opDetailsResp, &opDetails); err != nil {
		return "", fmt.Errorf("failed to parse operation details: %w", err)
	}

	params, err = c.resolveRefSvcParamValues(ctx, opDetails.InstanceOperation.CfsParams, params)
	if err != nil {
		return "", err
	}

	sent := make(map[int]bool)
	for paramId, value := range params {
		pPayload := genericParamReq{
			InstanceOperationUid:   opUid,
			SvcOperationCfsParamId: paramId,
			ParamValue:             value,
		}
		_, _, err := c.doRequest(ctx, "POST", "/instanceOperationCfsParams", pPayload)
		if err != nil {
			return "", fmt.Errorf("failed to set param %d: %w", paramId, err)
		}
		sent[paramId] = true
	}

	for _, param := range opDetails.InstanceOperation.CfsParams {
		if sent[param.SvcOperationCfsParamId] {
			continue
		}
		if !param.IsRequired {
			continue
		}

		val := ""
		if param.ParamValue != nil {
			val = *param.ParamValue
		} else if param.DefaultValue != nil {
			val = *param.DefaultValue
		}
		val = normalizeUniversalValueV6(val, param)
		if !param.IsRequired && strings.TrimSpace(val) == "" {
			continue
		}

		pPayload := genericParamReq{
			InstanceOperationUid:   opUid,
			SvcOperationCfsParamId: param.SvcOperationCfsParamId,
			ParamValue:             val,
		}
		_, _, err := c.doRequest(ctx, "POST", "/instanceOperationCfsParams", pPayload)
		if err != nil {
			return "", fmt.Errorf("failed to submit default param %d: %w", param.SvcOperationCfsParamId, err)
		}
	}

	_, _, err = c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s/validate-cfs", opUid), nil)
	if err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}

	_, _, err = c.doRequest(ctx, "POST", fmt.Sprintf("/instanceOperations/%s/run", opUid), map[string]interface{}{})
	if err != nil {
		return "", fmt.Errorf("execution failed: %w", err)
	}

	// НЕ МЕНЯТЬ: завершение операции определяется по dtFinish
	if err := c.waitForOperationFinish(ctx, opUid, defaultOperationTimeout); err != nil {
		return "", err
	}
	if err := c.ensureInstanceCreated(ctx, instanceUid); err != nil {
		return "", err
	}

	return instanceUid, nil
}

func normalizeUniversalValueV6(val string, param universalCfsParam) string {
	trimmed := strings.TrimSpace(val)
	if strings.EqualFold(trimmed, "null") {
		trimmed = ""
	}
	if trimmed == "\"\"" {
		trimmed = ""
	}
	if trimmed != "" {
		return trimmed
	}

	dataType := strings.ToLower(param.DataType)
	nameHint := strings.ToLower(param.Name + " " + param.Code + " " + param.Label + " " + param.SvcOperationCfsParam)

	if strings.Contains(dataType, "array") || strings.Contains(nameHint, "array") || strings.Contains(nameHint, "list") {
		return "[]"
	}
	if strings.Contains(dataType, "map") || strings.Contains(dataType, "json") || strings.Contains(nameHint, "map") || strings.Contains(nameHint, "json") {
		return "{}"
	}

	return trimmed
}

// RunInstanceOperationUniversal runs an available operation (modify/suspend/delete/resume) if possible.
func (c *UniversalClient) RunInstanceOperationUniversal(ctx context.Context, instanceUid string, action string, params map[int]string) error {
	state, err := c.GetInstanceState(ctx, instanceUid)
	if err != nil {
		return err
	}
	if state.OperationIsPending || state.OperationIsInProgress {
		if err := c.waitForInstanceIdle(ctx, instanceUid, defaultOperationTimeout); err != nil {
			return err
		}
	}

	var opId int
	for _, op := range state.AvailableOperations {
		if strings.EqualFold(op.Operation, action) {
			opId = op.SvcOperationId
			break
		}
	}
	if opId == 0 {
		return fmt.Errorf("action %s not available for instance %s", action, instanceUid)
	}

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

	if params != nil {
		for paramId, value := range params {
			pPayload := genericParamReq{
				InstanceOperationUid:   opUid,
				SvcOperationCfsParamId: paramId,
				ParamValue:             value,
			}
			_, _, err := c.doRequest(ctx, "POST", "/instanceOperationCfsParams", pPayload)
			if err != nil {
				return fmt.Errorf("failed to set param %d: %w", paramId, err)
			}
		}
	}

	_, _, err = c.doRequest(ctx, "POST", fmt.Sprintf("/instanceOperations/%s/run", opUid), map[string]interface{}{})
	if err != nil {
		return err
	}

	// НЕ МЕНЯТЬ: завершение операции определяется по dtFinish
	return c.waitForOperationFinish(ctx, opUid, defaultOperationTimeout)
}

// RunInstanceOperationUniversalWithDefaults runs an operation and submits required params (including defaults).
func (c *UniversalClient) RunInstanceOperationUniversalWithDefaults(ctx context.Context, instanceUid string, action string, params map[int]string) error {
	state, err := c.GetInstanceState(ctx, instanceUid)
	if err != nil {
		return err
	}
	if state.OperationIsPending || state.OperationIsInProgress {
		if err := c.waitForInstanceIdle(ctx, instanceUid, defaultOperationTimeout); err != nil {
			return err
		}
	}

	var opId int
	for _, op := range state.AvailableOperations {
		if strings.EqualFold(op.Operation, action) {
			opId = op.SvcOperationId
			break
		}
	}
	if opId == 0 {
		return fmt.Errorf("action %s not available for instance %s", action, instanceUid)
	}

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

	opDetailsResp, _, err := c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s?fields=cfsParams", opUid), nil)
	if err != nil {
		return fmt.Errorf("failed to get operation details: %w", err)
	}
	var opDetails universalOpResponse
	if err := json.Unmarshal(opDetailsResp, &opDetails); err != nil {
		return fmt.Errorf("failed to parse operation details: %w", err)
	}

	params, err = c.resolveRefSvcParamValues(ctx, opDetails.InstanceOperation.CfsParams, params)
	if err != nil {
		return err
	}

	sent := make(map[int]bool)
	if params != nil {
		for paramId, value := range params {
			pPayload := genericParamReq{
				InstanceOperationUid:   opUid,
				SvcOperationCfsParamId: paramId,
				ParamValue:             value,
			}
			_, _, err := c.doRequest(ctx, "POST", "/instanceOperationCfsParams", pPayload)
			if err != nil {
				return fmt.Errorf("failed to set param %d: %w", paramId, err)
			}
			sent[paramId] = true
		}
	}

	for _, param := range opDetails.InstanceOperation.CfsParams {
		if sent[param.SvcOperationCfsParamId] {
			continue
		}

		val := ""
		if param.ParamValue != nil {
			val = *param.ParamValue
		} else if param.DefaultValue != nil {
			val = *param.DefaultValue
		}
		val = normalizeUniversalValueV6(val, param)

		pPayload := genericParamReq{
			InstanceOperationUid:   opUid,
			SvcOperationCfsParamId: param.SvcOperationCfsParamId,
			ParamValue:             val,
		}
		_, _, err := c.doRequest(ctx, "POST", "/instanceOperationCfsParams", pPayload)
		if err != nil {
			return fmt.Errorf("failed to submit default param %d: %w", param.SvcOperationCfsParamId, err)
		}
	}

	_, _, err = c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s/validate-cfs", opUid), nil)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	_, _, err = c.doRequest(ctx, "POST", fmt.Sprintf("/instanceOperations/%s/run", opUid), map[string]interface{}{})
	if err != nil {
		return err
	}

	// НЕ МЕНЯТЬ: завершение операции определяется по dtFinish
	return c.waitForOperationFinish(ctx, opUid, defaultOperationTimeout)
}

// Instance state structures

type ApiOperation struct {
	SvcOperationId int    `json:"svcOperationId"`
	Operation      string `json:"operation"`
}

type InstanceStateResponse struct {
	InstanceUid           string         `json:"instanceUid"`
	ExplainedStatus       string         `json:"explainedStatus"`
	IsDeleted             bool           `json:"isDeleted"`
	OperationIsInProgress bool           `json:"operationIsInProgress"`
	OperationIsPending    bool           `json:"operationIsPending"`
	AvailableOperations   []ApiOperation `json:"availableOperations"`
}

// FindInstanceByDisplayName finds an instance by display_name for a given serviceId.
func (c *UniversalClient) FindInstanceByDisplayName(ctx context.Context, serviceId int, displayName string) (*InstanceStateResponse, error) {
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
		if resp.Body != nil {
			defer resp.Body.Close()
		}

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("status %d", resp.StatusCode)
		}

		var res struct {
			Results []struct {
				InstanceUid string `json:"instanceUid"`
				DisplayName string `json:"displayName"`
				ServiceId   int    `json:"serviceId"`
			} `json:"results"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return nil, err
		}

		if len(res.Results) == 0 {
			break
		}

		for _, item := range res.Results {
			if item.ServiceId == serviceId && item.DisplayName == displayName {
				state, err := c.GetInstanceState(ctx, item.InstanceUid)
				if err != nil {
					return nil, err
				}
				if state != nil && isInstanceDeleted(state) {
					continue
				}
				return state, nil
			}
		}

		page++
		if page > 100 {
			break
		}
	}

	return nil, nil
}

func (c *UniversalClient) GetInstanceState(ctx context.Context, instanceUid string) (*InstanceStateResponse, error) {
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

	if err := validateInstanceStatus(&res.Instance); err != nil {
		return nil, err
	}

	return &res.Instance, nil
}

func isInstanceDeleted(state *InstanceStateResponse) bool {
	if state == nil {
		return false
	}
	if state.IsDeleted {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(state.ExplainedStatus), "deleted")
}

// ===== UNIVERSAL OPERATION WAIT (APPEND-ONLY) =====
//
// КРИТИЧЕСКИ ВАЖНО:
//  1. Критерий завершения операции — наличие dtFinish.
//  2. Эта логика едина для всех сервисов/операций и является контрактом поведения.
//  3. ЗАПРЕЩЕНО ПРАВИТЬ ЭТОТ КОД БЕЗ ЯВНОГО СОГЛАСОВАНИЯ С ОПЕРАТОРОМ.
//     Любые изменения (таймауты, критерии завершения, частота опроса, обработка ошибок)
//     должны быть согласованы заранее.
const defaultOperationTimeout = 30 * time.Minute

type operationStatusResponse struct {
	InstanceOperation struct {
		DtFinish     *string `json:"dtFinish"`
		IsSuccessful *bool   `json:"isSuccessful"`
		ErrorLog     *string `json:"errorLog"`
		IsInProgress bool    `json:"isInProgress"`
		IsPending    bool    `json:"isPending"`
	} `json:"instanceOperation"`
}

func (c *UniversalClient) waitForOperationFinish(ctx context.Context, opUid string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation %s cancelled", opUid)
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for operation %s to finish", opUid)
			}

			respBody, _, err := c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s", opUid), nil)
			if err != nil {
				return fmt.Errorf("failed to check operation %s status: %w", opUid, err)
			}

			var status operationStatusResponse
			if err := json.Unmarshal(respBody, &status); err != nil {
				return fmt.Errorf("failed to parse operation %s status: %w", opUid, err)
			}

			// Критерий завершения — dtFinish (НЕ МЕНЯТЬ)
			if status.InstanceOperation.DtFinish != nil && strings.TrimSpace(*status.InstanceOperation.DtFinish) != "" {
				if status.InstanceOperation.IsSuccessful != nil && !*status.InstanceOperation.IsSuccessful {
					if status.InstanceOperation.ErrorLog != nil && strings.TrimSpace(*status.InstanceOperation.ErrorLog) != "" {
						return fmt.Errorf("operation %s failed: %s", opUid, *status.InstanceOperation.ErrorLog)
					}
					return fmt.Errorf("operation %s failed", opUid)
				}
				return nil
			}
		}
	}
}

// waitForInstanceIdle waits until no operation is pending/in-progress for the instance.
func (c *UniversalClient) waitForInstanceIdle(ctx context.Context, instanceUid string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation wait cancelled for instance %s", instanceUid)
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for instance %s to become idle", instanceUid)
			}

			state, err := c.GetInstanceState(ctx, instanceUid)
			if err != nil {
				return fmt.Errorf("failed to check instance %s state: %w", instanceUid, err)
			}
			if !state.OperationIsPending && !state.OperationIsInProgress {
				return nil
			}
		}
	}
}

// Internal HTTP helpers

func (c *UniversalClient) doRequest(ctx context.Context, method, path string, payload interface{}) ([]byte, http.Header, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, err
		}
		body = bytes.NewBuffer(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.ApiEndpoint+path, body)
	if err != nil {
		return nil, nil, err
	}

	req.Close = true
	req.Header.Set("Content-Type", "application/json")
	if c.ApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.ApiToken)
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, resp.Header, nil
}

func (c *UniversalClient) postIgnoreResponse(ctx context.Context, path string, payload interface{}, returnLocation bool) (string, error) {
	respBody, headers, err := c.doRequest(ctx, "POST", path, payload)
	if err != nil {
		return "", err
	}

	if returnLocation {
		if loc := headers.Get("Location"); loc != "" {
			return extractUIDFromLocation(loc), nil
		}
	}

	var justId string
	if err := json.Unmarshal(respBody, &justId); err == nil && justId != "" {
		return justId, nil
	}

	return "", nil
}

func extractUIDFromLocation(loc string) string {
	if loc == "" {
		return ""
	}
	return strings.TrimPrefix(loc, "./")
}

// NOTE(resourceRealm): доступные resourceRealm для сервиса получают через
// GET /resourceRealms/available?svcId=<serviceId> (через index.cfm proxy).
// Пример: svcId=1 (dummy) возвращает results="dummy".

// NOTE(refSvcId lookup): если параметр операции create ссылается на другой сервис,
// то в cfsParams будет проставлен refSvcId. Это означает, что значение параметра
// должно быть UUID инстанса указанного сервиса.
// Пример: s3UserUid (service_id=13, S3 бакет) имеет refSvcId=12 (S3 Object Storage),
// значит поле s3_user_uid должно быть UUID S3-инстанса, а не имя вида "s3-111805".
// Универсальный способ найти UUID по имени: запросить список инстансов
// GET /instances?page=N&size=100 и найти запись с displayName == <имя>
// и serviceId == refSvcId. Этот подход применим для любых сервисов (например, Postgres).

func (c *UniversalClient) resolveRefSvcParamValues(ctx context.Context, opParams []universalCfsParam, params map[int]string) (map[int]string, error) {
	if len(params) == 0 || len(opParams) == 0 {
		return params, nil
	}

	refById := make(map[int]int)
	for _, p := range opParams {
		if p.RefSvcId != nil && *p.RefSvcId > 0 {
			refById[p.SvcOperationCfsParamId] = *p.RefSvcId
		}
	}
	if len(refById) == 0 {
		return params, nil
	}

	resolved := make(map[int]string, len(params))
	for paramId, value := range params {
		newValue := value
		if refSvcId, ok := refById[paramId]; ok {
			if !isUUIDLike(value) {
				uid, err := c.findInstanceUidByDisplayNameRefSvc(ctx, refSvcId, value)
				if err != nil {
					return nil, err
				}
				if uid == "" {
					return nil, fmt.Errorf("failed to resolve param %d: no instance found for serviceId=%d displayName=%s", paramId, refSvcId, value)
				}
				newValue = uid
			}
		}
		resolved[paramId] = newValue
	}

	return resolved, nil
}

func (c *UniversalClient) findInstanceUidByDisplayNameRefSvc(ctx context.Context, serviceId int, displayName string) (string, error) {
	page := 1
	for {
		path := fmt.Sprintf("/instances?page=%d&size=100", page)
		respBody, _, err := c.doRequest(ctx, "GET", path, nil)
		if err != nil {
			return "", err
		}

		var res struct {
			Results []struct {
				InstanceUid string `json:"instanceUid"`
				DisplayName string `json:"displayName"`
				ServiceId   int    `json:"serviceId"`
			} `json:"results"`
		}
		if err := json.Unmarshal(respBody, &res); err != nil {
			return "", err
		}

		if len(res.Results) == 0 {
			break
		}

		for _, item := range res.Results {
			if item.ServiceId == serviceId && strings.EqualFold(item.DisplayName, displayName) {
				return item.InstanceUid, nil
			}
		}

		page++
		if page > 100 {
			break
		}
	}

	return "", nil
}

func isUUIDLike(value string) bool {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) != 36 {
		return false
	}
	for i, r := range trimmed {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !isHexDigit(r) {
				return false
			}
		}
	}
	return true
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func (c *UniversalClient) ensureInstanceCreated(ctx context.Context, instanceUid string) error {
	state, err := c.GetInstanceState(ctx, instanceUid)
	if err != nil {
		return fmt.Errorf("failed to fetch instance state after create: %w", err)
	}
	if state == nil {
		return fmt.Errorf("missing instance state after create for %s", instanceUid)
	}
	if state.IsDeleted {
		return fmt.Errorf("instance %s is deleted after create", instanceUid)
	}
	status := strings.ToLower(strings.TrimSpace(state.ExplainedStatus))
	if status == "not created" {
		return fmt.Errorf("instance %s not created after operation finish", instanceUid)
	}
	return nil
}

func validateInstanceStatus(state *InstanceStateResponse) error {
	if state == nil {
		return fmt.Errorf("missing instance state")
	}
	if state.IsDeleted {
		return fmt.Errorf("instance %s is deleted", state.InstanceUid)
	}
	if state.OperationIsPending || state.OperationIsInProgress {
		return fmt.Errorf("instance %s not ready: operation pending", state.InstanceUid)
	}
	status := strings.ToLower(strings.TrimSpace(state.ExplainedStatus))
	if strings.Contains(status, "not created") {
		return fmt.Errorf("instance %s not created", state.InstanceUid)
	}
	if strings.Contains(status, "pending") {
		return fmt.Errorf("instance %s pending", state.InstanceUid)
	}
	if strings.Contains(status, "failed") || strings.Contains(status, "error") {
		return fmt.Errorf("instance %s failed: %s", state.InstanceUid, state.ExplainedStatus)
	}
	return nil
}
