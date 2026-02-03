package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Param struct {
	ID       int    `yaml:"id"`
	Code     string `yaml:"code"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
	Default  string `yaml:"default"`
}

type ServiceYAML struct {
	Name      string `yaml:"name"`
	ServiceID int    `yaml:"service_id"`
	Create    struct {
		Params []Param `yaml:"params"`
	} `yaml:"create"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: min_tf <service_name>")
		os.Exit(1)
	}
	name := strings.TrimSpace(os.Args[1])
	if name == "" {
		fmt.Fprintln(os.Stderr, "service_name is required")
		os.Exit(1)
	}

	root, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	yamlPath := filepath.Join(root, "resources_yaml", fmt.Sprintf("%s.yaml", name))
	b, err := os.ReadFile(yamlPath)
	if err != nil {
		panic(err)
	}

	var svc ServiceYAML
	if err := yaml.Unmarshal(b, &svc); err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("resource \"nubes_%s\" \"test_%s\" {\n", name, name))
	buf.WriteString(fmt.Sprintf("  resource_name = \"tf-%s-devops\"\n", name))

	for _, p := range svc.Create.Params {
		if !p.Required {
			continue
		}
		attr := toSnake(p.Code)
		value := minimalValue(p)
		buf.WriteString(fmt.Sprintf("  %s = %s\n", attr, value))
	}
	buf.WriteString("}\n")

	fmt.Print(buf.String())
}

func minimalValue(p Param) string {
	if p.Default != "" {
		return formatValue(p.Type, p.Default)
	}
	switch strings.ToLower(p.Type) {
	case "bool":
		return "false"
	case "int", "int64", "number":
		return "1"
	default:
		return strconv.Quote("REPLACE_ME")
	}
}

func formatValue(t, v string) string {
	switch strings.ToLower(t) {
	case "bool":
		return strings.ToLower(strings.TrimSpace(v))
	case "int", "int64", "number":
		return strings.TrimSpace(v)
	default:
		return strconv.Quote(v)
	}
}

func toSnake(s string) string {
	var out []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out = append(out, '_')
		}
		out = append(out, rune(strings.ToLower(string(r))[0]))
	}
	return string(out)
}
