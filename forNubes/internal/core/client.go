package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func debugLog(format string, args ...interface{}) {
	f, err := os.OpenFile("/home/naeel/terra/debug_nubes.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, time.Now().Format(time.RFC3339)+": "+format+"\n", args...)
}

// UniversalClient handles Nubes API logic
type UniversalClient struct {
	HttpClient  *http.Client
	ApiEndpoint string
	ApiToken    string
}

// Request models
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

func (c *UniversalClient) CreateGenericInstance(ctx context.Context, serviceId int, displayName string, params map[int]string) (string, error) {
	// 1. Create Placeholder (POST /instances)
	tflog.Debug(ctx, fmt.Sprintf("Creating generic instance placeholder for service %d", serviceId))

	instPayload := genericInstanceReq{
		ServiceId:   serviceId,
		DisplayName: displayName,
		Descr:       "Created via Terraform Universal Provider",
	}

	instResp, instHeaders, err := c.doRequest(ctx, "POST", "/instances", instPayload)
	if err != nil {
		return "", err
	}
	debugLog("Step 1 Resp Body: %s", string(instResp))
	debugLog("Step 1 Resp Headers: %v", instHeaders)

	var instanceUid string

	// Try Location header first
	if loc := instHeaders.Get("Location"); loc != "" {
		// Location usually looks like "./3A021B21-..."
		instanceUid = strings.TrimPrefix(loc, "./")
	}

	// If header didn't give us ID, try body
	if instanceUid == "" {
		var instResult struct {
			InstanceUid string `json:"instanceUid"`
		}
		if err := json.Unmarshal(instResp, &instResult); err == nil && instResult.InstanceUid != "" {
			instanceUid = instResult.InstanceUid
		} else {
			// Fallback: maybe the body IS the ID (string)?
			var justId string
			if err2 := json.Unmarshal(instResp, &justId); err2 == nil && justId != "" {
				instanceUid = justId
			}
		}
	}

	if instanceUid == "" {
		return "", fmt.Errorf("could not extract instanceUid from response (Header: %s, Body: %s)", instHeaders.Get("Location"), string(instResp))
	}

	// 2. Init Operation (POST /instanceOperations)
	tflog.Debug(ctx, fmt.Sprintf("Initializing create operation for %s", instanceUid))
	opPayload := genericOpReq{
		InstanceUid: instanceUid,
		Operation:   "create",
	}

	opResp, opHeaders, err := c.doRequest(ctx, "POST", "/instanceOperations", opPayload)
	if err != nil {
		return "", err
	}
	debugLog("Step 2 Resp Body: %s", string(opResp))
	debugLog("Step 2 Resp Headers: %v", opHeaders)

	var opUid string

	if loc := opHeaders.Get("Location"); loc != "" {
		opUid = strings.TrimPrefix(loc, "./")
	}

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
		return "", fmt.Errorf("failed to extract instanceOperationUid from response (Header: %s, Body: %s)", opHeaders.Get("Location"), string(opResp))
	}

	// 3. Send Parameters (POST /instanceOperationCfsParams)
	for paramId, value := range params {
		tflog.Debug(ctx, fmt.Sprintf("Sending param %d = %s", paramId, value))

		pPayload := genericParamReq{
			InstanceOperationUid:   opUid,
			SvcOperationCfsParamId: paramId,
			ParamValue:             value,
		}

		_, _, err := c.doRequest(ctx, "POST", "/instanceOperationCfsParams", pPayload)
		if err != nil {
			debugLog("Step 3 Failed for param %d. Error: %v", paramId, err)
			return "", fmt.Errorf("failed to set param %d: %w", paramId, err)
		}
		debugLog("Step 3 OK for param %d", paramId)
	}

	// 4. Validate (GET /validate-cfs)
	tflog.Debug(ctx, fmt.Sprintf("Validating operation %s", opUid))
	_, _, err = c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s/validate-cfs", opUid), nil)
	if err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}

	// 5. Execute (POST /run)
	tflog.Debug(ctx, fmt.Sprintf("Executing operation %s", opUid))
	_, _, err = c.doRequest(ctx, "POST", fmt.Sprintf("/instanceOperations/%s/run", opUid), map[string]interface{}{})
	if err != nil {
		return "", fmt.Errorf("execution failed: %w", err)
	}

	// 6. Wait for completion (Simple Polling)
	tflog.Info(ctx, "Waiting for resource creation...")
	time.Sleep(5 * time.Second)

	return instanceUid, nil
}

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

	// Force close connection
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

// ===== UNIVERSAL FLOW (APPEND-ONLY) =====

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
	DataType               string  `json:"dataType"`
}

// CreateGenericInstanceUniversal реализует универсальный Nubes Flow:
// instances -> instanceOperations -> get cfsParams -> submit params -> validate -> run
func (c *UniversalClient) CreateGenericInstanceUniversal(ctx context.Context, serviceId int, displayName string, params map[int]string) (string, error) {
	// 1. Create Placeholder (POST /instances)
	instPayload := genericInstanceReq{
		ServiceId:   serviceId,
		DisplayName: displayName,
		Descr:       "Created via Terraform Universal Provider",
	}

	instResp, instHeaders, err := c.doRequest(ctx, "POST", "/instances", instPayload)
	if err != nil {
		return "", err
	}

	var instanceUid string
	if loc := instHeaders.Get("Location"); loc != "" {
		instanceUid = strings.TrimPrefix(loc, "./")
	}
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
		return "", fmt.Errorf("could not extract instanceUid from response (Header: %s, Body: %s)", instHeaders.Get("Location"), string(instResp))
	}

	// 2. Init Operation (POST /instanceOperations)
	opPayload := genericOpReq{
		InstanceUid: instanceUid,
		Operation:   "create",
	}

	opResp, opHeaders, err := c.doRequest(ctx, "POST", "/instanceOperations", opPayload)
	if err != nil {
		return "", err
	}

	var opUid string
	if loc := opHeaders.Get("Location"); loc != "" {
		opUid = strings.TrimPrefix(loc, "./")
	}
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
		return "", fmt.Errorf("failed to extract instanceOperationUid from response (Header: %s, Body: %s)", opHeaders.Get("Location"), string(opResp))
	}

	// 3. Get operation details (cfsParams)
	opDetailsResp, _, err := c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s?fields=cfsParams", opUid), nil)
	if err != nil {
		return "", fmt.Errorf("failed to get operation details: %w", err)
	}
	var opDetails universalOpResponse
	if err := json.Unmarshal(opDetailsResp, &opDetails); err != nil {
		return "", fmt.Errorf("failed to parse operation details: %w", err)
	}

	// 4. Submit explicit params
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

	// 5. Submit defaults for missing params
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
		val = normalizeUniversalValue(val, param.DataType)

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

	// 6. Validate (GET /validate-cfs)
	_, _, err = c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s/validate-cfs", opUid), nil)
	if err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}

	// 7. Execute (POST /run)
	_, _, err = c.doRequest(ctx, "POST", fmt.Sprintf("/instanceOperations/%s/run", opUid), map[string]interface{}{})
	if err != nil {
		return "", fmt.Errorf("execution failed: %w", err)
	}

	// 8. Wait for completion (Simple Polling)
	time.Sleep(5 * time.Second)

	return instanceUid, nil
}

func normalizeUniversalValue(val string, dataType string) string {
	if val != "" {
		return val
	}

	switch strings.ToLower(dataType) {
	case "map", "json":
		return "{}"
	case "array", "list":
		return "[]"
	default:
		return val
	}
}

// ===== UNIVERSAL FLOW V2 (APPEND-ONLY) =====

type universalOpResponseV2 struct {
	InstanceOperation universalOperationV2 `json:"instanceOperation"`
}

type universalOperationV2 struct {
	CfsParams []universalCfsParamV2 `json:"cfsParams"`
}

type universalCfsParamV2 struct {
	SvcOperationCfsParamId int     `json:"svcOperationCfsParamId"`
	ParamValue             *string `json:"paramValue"`
	DefaultValue           *string `json:"defaultValue"`
	DataType               string  `json:"dataType"`
	Name                   string  `json:"name"`
	Code                   string  `json:"code"`
	SvcOperationCfsParam   string  `json:"svcOperationCfsParam"`
}

// CreateGenericInstanceUniversalV2 - расширенная нормализация значений
// (map/json/list/array) с учетом name/code/svcOperationCfsParam
func (c *UniversalClient) CreateGenericInstanceUniversalV2(ctx context.Context, serviceId int, displayName string, params map[int]string) (string, error) {
	// 1. Create Placeholder (POST /instances)
	instPayload := genericInstanceReq{
		ServiceId:   serviceId,
		DisplayName: displayName,
		Descr:       "Created via Terraform Universal Provider",
	}

	instResp, instHeaders, err := c.doRequest(ctx, "POST", "/instances", instPayload)
	if err != nil {
		return "", err
	}

	var instanceUid string
	if loc := instHeaders.Get("Location"); loc != "" {
		instanceUid = strings.TrimPrefix(loc, "./")
	}
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
		return "", fmt.Errorf("could not extract instanceUid from response (Header: %s, Body: %s)", instHeaders.Get("Location"), string(instResp))
	}

	// 2. Init Operation (POST /instanceOperations)
	opPayload := genericOpReq{
		InstanceUid: instanceUid,
		Operation:   "create",
	}

	opResp, opHeaders, err := c.doRequest(ctx, "POST", "/instanceOperations", opPayload)
	if err != nil {
		return "", err
	}

	var opUid string
	if loc := opHeaders.Get("Location"); loc != "" {
		opUid = strings.TrimPrefix(loc, "./")
	}
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
		return "", fmt.Errorf("failed to extract instanceOperationUid from response (Header: %s, Body: %s)", opHeaders.Get("Location"), string(opResp))
	}

	// 3. Get operation details (cfsParams)
	opDetailsResp, _, err := c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s?fields=cfsParams", opUid), nil)
	if err != nil {
		return "", fmt.Errorf("failed to get operation details: %w", err)
	}
	var opDetails universalOpResponseV2
	if err := json.Unmarshal(opDetailsResp, &opDetails); err != nil {
		return "", fmt.Errorf("failed to parse operation details: %w", err)
	}

	// 4. Submit explicit params
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

	// 5. Submit defaults for missing params
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
		val = normalizeUniversalValueV2(val, param)

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

	// 6. Validate (GET /validate-cfs)
	_, _, err = c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s/validate-cfs", opUid), nil)
	if err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}

	// 7. Execute (POST /run)
	_, _, err = c.doRequest(ctx, "POST", fmt.Sprintf("/instanceOperations/%s/run", opUid), map[string]interface{}{})
	if err != nil {
		return "", fmt.Errorf("execution failed: %w", err)
	}

	// 8. Wait for completion (Simple Polling)
	time.Sleep(5 * time.Second)

	return instanceUid, nil
}

func normalizeUniversalValueV2(val string, param universalCfsParamV2) string {
	if val != "" {
		return val
	}

	dataType := strings.ToLower(param.DataType)
	nameHint := strings.ToLower(param.Name + " " + param.Code + " " + param.SvcOperationCfsParam)

	if dataType == "map" || dataType == "json" || strings.Contains(nameHint, "map") || strings.Contains(nameHint, "json") {
		return "{}"
	}
	if dataType == "array" || dataType == "list" || strings.Contains(nameHint, "array") || strings.Contains(nameHint, "list") {
		return "[]"
	}

	return val
}

// ===== UNIVERSAL FLOW V3 (APPEND-ONLY) =====

type universalOpResponseV3 struct {
	InstanceOperation universalOperationV3 `json:"instanceOperation"`
}

type universalOperationV3 struct {
	CfsParams []universalCfsParamV3 `json:"cfsParams"`
}

type universalCfsParamV3 struct {
	SvcOperationCfsParamId int     `json:"svcOperationCfsParamId"`
	ParamValue             *string `json:"paramValue"`
	DefaultValue           *string `json:"defaultValue"`
	DataType               string  `json:"dataType"`
	Name                   string  `json:"name"`
	Code                   string  `json:"code"`
	Label                  string  `json:"label"`
	SvcOperationCfsParam   string  `json:"svcOperationCfsParam"`
}

// CreateGenericInstanceUniversalV3 - расширенная нормализация (label/name/code)
func (c *UniversalClient) CreateGenericInstanceUniversalV3(ctx context.Context, serviceId int, displayName string, params map[int]string) (string, error) {
	// 1. Create Placeholder (POST /instances)
	instPayload := genericInstanceReq{
		ServiceId:   serviceId,
		DisplayName: displayName,
		Descr:       "Created via Terraform Universal Provider",
	}

	instResp, instHeaders, err := c.doRequest(ctx, "POST", "/instances", instPayload)
	if err != nil {
		return "", err
	}

	var instanceUid string
	if loc := instHeaders.Get("Location"); loc != "" {
		instanceUid = strings.TrimPrefix(loc, "./")
	}
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
		return "", fmt.Errorf("could not extract instanceUid from response (Header: %s, Body: %s)", instHeaders.Get("Location"), string(instResp))
	}

	// 2. Init Operation (POST /instanceOperations)
	opPayload := genericOpReq{
		InstanceUid: instanceUid,
		Operation:   "create",
	}

	opResp, opHeaders, err := c.doRequest(ctx, "POST", "/instanceOperations", opPayload)
	if err != nil {
		return "", err
	}

	var opUid string
	if loc := opHeaders.Get("Location"); loc != "" {
		opUid = strings.TrimPrefix(loc, "./")
	}
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
		return "", fmt.Errorf("failed to extract instanceOperationUid from response (Header: %s, Body: %s)", opHeaders.Get("Location"), string(opResp))
	}

	// 3. Get operation details (cfsParams)
	opDetailsResp, _, err := c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s?fields=cfsParams", opUid), nil)
	if err != nil {
		return "", fmt.Errorf("failed to get operation details: %w", err)
	}
	var opDetails universalOpResponseV3
	if err := json.Unmarshal(opDetailsResp, &opDetails); err != nil {
		return "", fmt.Errorf("failed to parse operation details: %w", err)
	}

	// Debug log of cfsParams (for type mismatch analysis)
	for _, p := range opDetails.InstanceOperation.CfsParams {
		debugLog("CFS Param: id=%d dataType=%s name=%s code=%s label=%s svcParam=%s", p.SvcOperationCfsParamId, p.DataType, p.Name, p.Code, p.Label, p.SvcOperationCfsParam)
	}

	// 4. Submit explicit params
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

	// 5. Submit defaults for missing params
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
		val = normalizeUniversalValueV3(val, param)

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

	// 6. Validate (GET /validate-cfs)
	_, _, err = c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s/validate-cfs", opUid), nil)
	if err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}

	// 7. Execute (POST /run)
	_, _, err = c.doRequest(ctx, "POST", fmt.Sprintf("/instanceOperations/%s/run", opUid), map[string]interface{}{})
	if err != nil {
		return "", fmt.Errorf("execution failed: %w", err)
	}

	// 8. Wait for completion (Simple Polling)
	time.Sleep(5 * time.Second)

	return instanceUid, nil
}

func normalizeUniversalValueV3(val string, param universalCfsParamV3) string {
	if val != "" {
		return val
	}

	dataType := strings.ToLower(param.DataType)
	nameHint := strings.ToLower(param.Name + " " + param.Code + " " + param.Label + " " + param.SvcOperationCfsParam)

	if dataType == "map" || dataType == "json" || strings.Contains(nameHint, "map") || strings.Contains(nameHint, "json") {
		return "{}"
	}
	if dataType == "array" || dataType == "list" || strings.Contains(nameHint, "array") || strings.Contains(nameHint, "list") {
		return "[]"
	}

	return val
}

// ===== UNIVERSAL FLOW V4 (APPEND-ONLY) =====

// CreateGenericInstanceUniversalV4 - нормализация с trim + расширенный разбор dataType
func (c *UniversalClient) CreateGenericInstanceUniversalV4(ctx context.Context, serviceId int, displayName string, params map[int]string) (string, error) {
	// 1. Create Placeholder (POST /instances)
	instPayload := genericInstanceReq{
		ServiceId:   serviceId,
		DisplayName: displayName,
		Descr:       "Created via Terraform Universal Provider",
	}

	instResp, instHeaders, err := c.doRequest(ctx, "POST", "/instances", instPayload)
	if err != nil {
		return "", err
	}

	var instanceUid string
	if loc := instHeaders.Get("Location"); loc != "" {
		instanceUid = strings.TrimPrefix(loc, "./")
	}
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
		return "", fmt.Errorf("could not extract instanceUid from response (Header: %s, Body: %s)", instHeaders.Get("Location"), string(instResp))
	}

	// 2. Init Operation (POST /instanceOperations)
	opPayload := genericOpReq{
		InstanceUid: instanceUid,
		Operation:   "create",
	}

	opResp, opHeaders, err := c.doRequest(ctx, "POST", "/instanceOperations", opPayload)
	if err != nil {
		return "", err
	}

	var opUid string
	if loc := opHeaders.Get("Location"); loc != "" {
		opUid = strings.TrimPrefix(loc, "./")
	}
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
		return "", fmt.Errorf("failed to extract instanceOperationUid from response (Header: %s, Body: %s)", opHeaders.Get("Location"), string(opResp))
	}

	// 3. Get operation details (cfsParams)
	opDetailsResp, _, err := c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s?fields=cfsParams", opUid), nil)
	if err != nil {
		return "", fmt.Errorf("failed to get operation details: %w", err)
	}
	var opDetails universalOpResponseV3
	if err := json.Unmarshal(opDetailsResp, &opDetails); err != nil {
		return "", fmt.Errorf("failed to parse operation details: %w", err)
	}

	// Debug log of cfsParams (for type mismatch analysis)
	for _, p := range opDetails.InstanceOperation.CfsParams {
		debugLog("CFS Param: id=%d dataType=%s name=%s code=%s label=%s svcParam=%s", p.SvcOperationCfsParamId, p.DataType, p.Name, p.Code, p.Label, p.SvcOperationCfsParam)
	}

	// 4. Submit explicit params
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

	// 5. Submit defaults for missing params
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
		val = normalizeUniversalValueV4(val, param)

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

	// 6. Validate (GET /validate-cfs)
	_, _, err = c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s/validate-cfs", opUid), nil)
	if err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}

	// 7. Execute (POST /run)
	_, _, err = c.doRequest(ctx, "POST", fmt.Sprintf("/instanceOperations/%s/run", opUid), map[string]interface{}{})
	if err != nil {
		return "", fmt.Errorf("execution failed: %w", err)
	}

	// 8. Wait for completion (Simple Polling)
	time.Sleep(5 * time.Second)

	return instanceUid, nil
}

func normalizeUniversalValueV4(val string, param universalCfsParamV3) string {
	trimmed := strings.TrimSpace(val)
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

// ===== UNIVERSAL FLOW V5 (APPEND-ONLY) =====

// CreateGenericInstanceUniversalV5 - добавляет логирование отправляемых значений
func (c *UniversalClient) CreateGenericInstanceUniversalV5(ctx context.Context, serviceId int, displayName string, params map[int]string) (string, error) {
	// 1. Create Placeholder (POST /instances)
	instPayload := genericInstanceReq{
		ServiceId:   serviceId,
		DisplayName: displayName,
		Descr:       "Created via Terraform Universal Provider",
	}

	instResp, instHeaders, err := c.doRequest(ctx, "POST", "/instances", instPayload)
	if err != nil {
		return "", err
	}

	var instanceUid string
	if loc := instHeaders.Get("Location"); loc != "" {
		instanceUid = strings.TrimPrefix(loc, "./")
	}
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
		return "", fmt.Errorf("could not extract instanceUid from response (Header: %s, Body: %s)", instHeaders.Get("Location"), string(instResp))
	}

	// 2. Init Operation (POST /instanceOperations)
	opPayload := genericOpReq{
		InstanceUid: instanceUid,
		Operation:   "create",
	}

	opResp, opHeaders, err := c.doRequest(ctx, "POST", "/instanceOperations", opPayload)
	if err != nil {
		return "", err
	}

	var opUid string
	if loc := opHeaders.Get("Location"); loc != "" {
		opUid = strings.TrimPrefix(loc, "./")
	}
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
		return "", fmt.Errorf("failed to extract instanceOperationUid from response (Header: %s, Body: %s)", opHeaders.Get("Location"), string(opResp))
	}

	// 3. Get operation details (cfsParams)
	opDetailsResp, _, err := c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s?fields=cfsParams", opUid), nil)
	if err != nil {
		return "", fmt.Errorf("failed to get operation details: %w", err)
	}
	var opDetails universalOpResponseV3
	if err := json.Unmarshal(opDetailsResp, &opDetails); err != nil {
		return "", fmt.Errorf("failed to parse operation details: %w", err)
	}

	// Debug log of cfsParams (for type mismatch analysis)
	for _, p := range opDetails.InstanceOperation.CfsParams {
		debugLog("CFS Param: id=%d dataType=%s name=%s code=%s label=%s svcParam=%s", p.SvcOperationCfsParamId, p.DataType, p.Name, p.Code, p.Label, p.SvcOperationCfsParam)
	}

	// 4. Submit explicit params
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

	// 5. Submit defaults for missing params
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
		normalized := normalizeUniversalValueV5(val, param)
		debugLog("CFS Send: id=%d raw=%q normalized=%q", param.SvcOperationCfsParamId, val, normalized)

		pPayload := genericParamReq{
			InstanceOperationUid:   opUid,
			SvcOperationCfsParamId: param.SvcOperationCfsParamId,
			ParamValue:             normalized,
		}
		_, _, err := c.doRequest(ctx, "POST", "/instanceOperationCfsParams", pPayload)
		if err != nil {
			return "", fmt.Errorf("failed to submit default param %d: %w", param.SvcOperationCfsParamId, err)
		}
	}

	// 6. Validate (GET /validate-cfs)
	_, _, err = c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s/validate-cfs", opUid), nil)
	if err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}

	// 7. Execute (POST /run)
	_, _, err = c.doRequest(ctx, "POST", fmt.Sprintf("/instanceOperations/%s/run", opUid), map[string]interface{}{})
	if err != nil {
		return "", fmt.Errorf("execution failed: %w", err)
	}

	// 8. Wait for completion (Simple Polling)
	time.Sleep(5 * time.Second)

	return instanceUid, nil
}

func normalizeUniversalValueV5(val string, param universalCfsParamV3) string {
	trimmed := strings.TrimSpace(val)
	if strings.EqualFold(trimmed, "null") {
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

// ===== UNIVERSAL FLOW V6 (APPEND-ONLY) =====

// CreateGenericInstanceUniversalV6 - обработка raw="\"\"" для map/json/array/list
func (c *UniversalClient) CreateGenericInstanceUniversalV6(ctx context.Context, serviceId int, displayName string, params map[int]string) (string, error) {
	// 1. Create Placeholder (POST /instances)
	instPayload := genericInstanceReq{
		ServiceId:   serviceId,
		DisplayName: displayName,
		Descr:       "Created via Terraform Universal Provider",
	}

	instResp, instHeaders, err := c.doRequest(ctx, "POST", "/instances", instPayload)
	if err != nil {
		return "", err
	}

	var instanceUid string
	if loc := instHeaders.Get("Location"); loc != "" {
		instanceUid = strings.TrimPrefix(loc, "./")
	}
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
		return "", fmt.Errorf("could not extract instanceUid from response (Header: %s, Body: %s)", instHeaders.Get("Location"), string(instResp))
	}

	// 2. Init Operation (POST /instanceOperations)
	opPayload := genericOpReq{
		InstanceUid: instanceUid,
		Operation:   "create",
	}

	opResp, opHeaders, err := c.doRequest(ctx, "POST", "/instanceOperations", opPayload)
	if err != nil {
		return "", err
	}

	var opUid string
	if loc := opHeaders.Get("Location"); loc != "" {
		opUid = strings.TrimPrefix(loc, "./")
	}
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
		return "", fmt.Errorf("failed to extract instanceOperationUid from response (Header: %s, Body: %s)", opHeaders.Get("Location"), string(opResp))
	}

	// 3. Get operation details (cfsParams)
	opDetailsResp, _, err := c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s?fields=cfsParams", opUid), nil)
	if err != nil {
		return "", fmt.Errorf("failed to get operation details: %w", err)
	}
	var opDetails universalOpResponseV3
	if err := json.Unmarshal(opDetailsResp, &opDetails); err != nil {
		return "", fmt.Errorf("failed to parse operation details: %w", err)
	}

	for _, p := range opDetails.InstanceOperation.CfsParams {
		debugLog("CFS Param: id=%d dataType=%s name=%s code=%s label=%s svcParam=%s", p.SvcOperationCfsParamId, p.DataType, p.Name, p.Code, p.Label, p.SvcOperationCfsParam)
	}

	// 4. Submit explicit params
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

	// 5. Submit defaults for missing params
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
		normalized := normalizeUniversalValueV6(val, param)
		debugLog("CFS Send: id=%d raw=%q normalized=%q", param.SvcOperationCfsParamId, val, normalized)

		pPayload := genericParamReq{
			InstanceOperationUid:   opUid,
			SvcOperationCfsParamId: param.SvcOperationCfsParamId,
			ParamValue:             normalized,
		}
		_, _, err := c.doRequest(ctx, "POST", "/instanceOperationCfsParams", pPayload)
		if err != nil {
			return "", fmt.Errorf("failed to submit default param %d: %w", param.SvcOperationCfsParamId, err)
		}
	}

	// 6. Validate (GET /validate-cfs)
	_, _, err = c.doRequest(ctx, "GET", fmt.Sprintf("/instanceOperations/%s/validate-cfs", opUid), nil)
	if err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}

	// 7. Execute (POST /run)
	_, _, err = c.doRequest(ctx, "POST", fmt.Sprintf("/instanceOperations/%s/run", opUid), map[string]interface{}{})
	if err != nil {
		return "", fmt.Errorf("execution failed: %w", err)
	}

	// 8. Wait for completion (Simple Polling)
	time.Sleep(5 * time.Second)

	return instanceUid, nil
}

func normalizeUniversalValueV6(val string, param universalCfsParamV3) string {
	trimmed := strings.TrimSpace(val)
	if strings.EqualFold(trimmed, "null") {
		trimmed = ""
	}
	// Handle JSON-encoded empty string: ""
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
