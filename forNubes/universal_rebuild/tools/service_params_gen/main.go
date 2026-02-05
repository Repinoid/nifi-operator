package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// service_params_gen: builds resource YAML from /index.cfm?endpoint=/services/{svcId}
// and /index.cfm?endpoint=/serviceOperation/{svcOperationId}.
//
// Env:
// - NUBES_API_TOKEN (or read from test_universal/terraform.tfvars)
// - NUBES_API_ENDPOINT (default: https://deck-api.ngcloud.ru/api/v1/index.cfm)
// - NUBES_SERVICE_ID (required)
// - NUBES_SERVICE_NAME (optional override for YAML name)
// - NUBES_OUTPUT (optional output path)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		panic(err)
	}

	client := &apiClient{
		endpoint: cfg.ApiEndpoint,
		token:    cfg.ApiToken,
	}

	service, err := client.getService(cfg.ServiceID)
	if err != nil {
		panic(err)
	}

	name := cfg.ServiceName
	if name == "" {
		name = service.Name
	}
	if name == "" {
		name = fmt.Sprintf("service_%d", cfg.ServiceID)
	}

	ops, err := client.collectOperations(service.Operations)
	if err != nil {
		panic(err)
	}

	spec := ResourceSpec{
		Name:      name,
		ServiceID: cfg.ServiceID,
		Create:    Operation{Params: ops["create"]},
		Modify:    Operation{Params: ops["modify"]},
		Lifecycle: Lifecycle{DeleteModeDefault: "state_only", ResumeIfExistsDefault: true},
	}

	out, err := yaml.Marshal(spec)
	if err != nil {
		panic(err)
	}

	if cfg.OutputPath != "" {
		if err := os.WriteFile(cfg.OutputPath, out, 0o644); err != nil {
			panic(err)
		}
		fmt.Printf("written %s\n", cfg.OutputPath)
		return
	}

	fmt.Print(string(out))
}

// ===== config =====

type config struct {
	ApiEndpoint string
	ApiToken    string
	ServiceID   int
	ServiceName string
	OutputPath  string
}

func loadConfig() (config, error) {
	apiEndpoint := getenvDefault("NUBES_API_ENDPOINT", "https://deck-api.ngcloud.ru/api/v1/index.cfm")
	apiToken := strings.TrimSpace(os.Getenv("NUBES_API_TOKEN"))
	if apiToken == "" {
		apiToken = strings.TrimSpace(readTokenFromTfvars("/home/naeel/terra/universal_rebuild/test_universal/terraform.tfvars"))
	}
	if apiToken == "" {
		return config{}, errors.New("NUBES_API_TOKEN is required")
	}

	serviceIDStr := strings.TrimSpace(os.Getenv("NUBES_SERVICE_ID"))
	if serviceIDStr == "" {
		return config{}, errors.New("NUBES_SERVICE_ID is required")
	}
	serviceID, err := strconv.Atoi(serviceIDStr)
	if err != nil {
		return config{}, fmt.Errorf("invalid NUBES_SERVICE_ID: %s", serviceIDStr)
	}

	name := strings.TrimSpace(os.Getenv("NUBES_SERVICE_NAME"))
	output := strings.TrimSpace(os.Getenv("NUBES_OUTPUT"))
	if output == "" {
		output = filepath.Join("/home/naeel/terra/universal_rebuild/resources_yaml", fmt.Sprintf("%s.yaml", nameOrDefault(name, serviceID)))
	}

	return config{
		ApiEndpoint: apiEndpoint,
		ApiToken:    apiToken,
		ServiceID:   serviceID,
		ServiceName: name,
		OutputPath:  output,
	}, nil
}

func nameOrDefault(name string, serviceID int) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return fmt.Sprintf("service_%d", serviceID)
}

func getenvDefault(key, def string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	return val
}

func readTokenFromTfvars(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	re := regexp.MustCompile(`(?m)^\s*api_token\s*=\s*"([^"]+)"`)
	m := re.FindStringSubmatch(string(b))
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// ===== api client =====

type apiClient struct {
	endpoint string
	token    string
}

type serviceResponse struct {
	Service serviceInfo `json:"svc"`
}

type serviceInfo struct {
	ID         int             `json:"svcId"`
	Name       string          `json:"svcShort"`
	Operations []operationInfo `json:"operations"`
}

type operationInfo struct {
	SvcOperationId int    `json:"svcOperationId"`
	Operation      string `json:"operation"`
}

type serviceOperationResponse struct {
	ServiceOperation serviceOperationInfo `json:"svcOperation"`
}

type serviceOperationInfo struct {
	SvcOperationId int        `json:"svcOperationId"`
	Operation      string     `json:"operation"`
	CfsParams      []cfsParam `json:"cfsParams"`
}

type cfsParam struct {
	ID           int         `json:"svcOperationCfsParamId"`
	Code         string      `json:"svcOperationCfsParam"`
	DataType     string      `json:"dataType"`
	IsRequired   bool        `json:"isRequired"`
	DefaultValue interface{} `json:"defaultValue"`
}

func (c *apiClient) getService(serviceID int) (serviceInfo, error) {
	endpoint := fmt.Sprintf("/services/%d", serviceID)
	var res serviceResponse
	if err := c.getViaProxy(endpoint, &res); err != nil {
		return serviceInfo{}, err
	}
	return res.Service, nil
}

func (c *apiClient) getServiceOperation(svcOperationId int) (serviceOperationInfo, error) {
	endpoint := fmt.Sprintf("/serviceOperation/%d", svcOperationId)
	var res serviceOperationResponse
	if err := c.getViaProxy(endpoint, &res); err != nil {
		return serviceOperationInfo{}, err
	}
	return res.ServiceOperation, nil
}

func (c *apiClient) getViaProxy(endpoint string, out interface{}) error {
	req, err := http.NewRequest("GET", c.endpoint, nil)
	if err != nil {
		return err
	}
	q := req.URL.Query()
	q.Set("endpoint", endpoint)
	req.URL.RawQuery = q.Encode()
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(out)
}

func (c *apiClient) collectOperations(ops []operationInfo) (map[string][]Param, error) {
	result := map[string][]Param{
		"create": {},
		"modify": {},
	}
	for _, op := range ops {
		name := strings.ToLower(strings.TrimSpace(op.Operation))
		if name != "create" && name != "modify" {
			continue
		}
		opInfo, err := c.getServiceOperation(op.SvcOperationId)
		if err != nil {
			return nil, err
		}
		params := make([]Param, 0, len(opInfo.CfsParams))
		for _, p := range opInfo.CfsParams {
			params = append(params, Param{
				ID:       p.ID,
				Code:     p.Code,
				Type:     mapType(p.DataType),
				Required: p.IsRequired,
				Default:  formatDefault(p.DefaultValue),
			})
		}
		sort.Slice(params, func(i, j int) bool { return params[i].ID < params[j].ID })
		result[name] = params
	}
	return result, nil
}

func mapType(dt string) string {
	d := strings.ToLower(strings.TrimSpace(dt))
	if strings.Contains(d, "bool") {
		return "bool"
	}
	if strings.Contains(d, "int") {
		return "int64"
	}
	return "string"
}

func formatDefault(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%v", t)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

// ===== spec =====

type ResourceSpec struct {
	Name      string    `yaml:"name"`
	ServiceID int       `yaml:"service_id"`
	Create    Operation `yaml:"create"`
	Modify    Operation `yaml:"modify"`
	Lifecycle Lifecycle `yaml:"lifecycle"`
}

type Operation struct {
	Params []Param `yaml:"params"`
}

type Lifecycle struct {
	DeleteModeDefault     string `yaml:"delete_mode_default"`
	ResumeIfExistsDefault bool   `yaml:"resume_if_exists_default"`
}

type Param struct {
	ID       int    `yaml:"id"`
	Code     string `yaml:"code"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
	Default  string `yaml:"default,omitempty"`
}
