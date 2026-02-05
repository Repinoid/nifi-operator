package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Token from terraform.tfvars (simplified)
const token = "eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJUdzZMa1FRS1ctZHlpb0hoYXlxOHRxa2dhb2RqQjM0bVAtSDBjUkpGWkRJIn0.eyJleHAiOjE3Njk4NDQxMTAsImlhdCI6MTc2OTgzNjkxMCwiYXV0aF90aW1lIjoxNzY5ODM2ODg5LCJqdGkiOiIzMGM2MjZlMC1kMDY2LTQzNTItODFiMC0yZGM4Yjk0ODM5NzIiLCJpc3MiOiJodHRwczovL2tleWNsb2FrLm51YmVzLnJ1L3JlYWxtcy9jbG91ZCIsImF1ZCI6ImFjY291bnQiLCJzdWIiOiJhNTVhNjFlZC1iMTU5LTQ3YjYtYmIxNi00N2Q4ZjIyZDQ4MGIiLCJ0eXAiOiJCZWFyZXIiLCJhenAiOiJhY2NvdW50LWNvbnNvbGUiLCJub25jZSI6IjM2ODc0MDljLWU1MGQtNGVkMi1iY2JkLTU1ODA2MmNlY2ZkNCIsInNlc3Npb25fc3RhdGUiOiI1YTYxMTdkMS03ZTI2LTQ3YTQtOWUxYS0xZTFlZTViYTlmNzQiLCJhY3IiOiIwIiwicmVzb3VyY2VfYWNjZXNzIjp7ImFjY291bnQiOnsicm9sZXMiOlsibWFuYWdlLWFjY291bnQiLCJtYW5hZ2UtYWNjb3VudC1saW5rcyJdfX0sInNjb3BlIjoib3BlbmlkIGVtYWlsIHByb2ZpbGUiLCJzaWQiOiI1YTYxMTdkMS03ZTI2LTQ3YTQtOWUxYS0xZTFlZTViYTlmNzQiLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwibmFtZSI6ItCi0LDQt9C10YLQtNC40L3QvtCyINCd0LDQuNC70YwiLCJDbGllbnRJRCI6IldaMDEzMjUiLCJwcmVmZXJyZWRfdXNlcm5hbWUiOiJ0YXpldEBuYXJvZC5ydSIsImxvY2FsZSI6InJ1IiwiZ2l2ZW5fbmFtZSI6ItCi0LDQt9C10YLQtNC40L3QvtCyIiwiZmFtaWx5X25hbWUiOiLQndCw0LjQu9GMIiwiZW1haWwiOiJ0YXpldEBuYXJvZC5ydSJ9.gqIGJ5aH1nqBqpnvVOtZWXMt3pZp5m264xQ3ar2D1rUOh0NHSFtoIKSjJkgSlmFeFMjd-C261MQDg78YwGzYn2RH8hLJq-ht0l07-ZGr9bqYNyjcNTyeQ_bjqA5sGBKzTd1XfA4jsuMqZ6rIZAsy_NnVunHodivHSSKv7LdArZeyTeOIFVojattX96CDiBLnM45utOtS0NM3Ae8rowKJWj9l46N4c8n8u-zJ6rx5eP50LOYjz1sod7lwcgatTATDHDwH0moYabzPdeOvK-xpbSmibMhKUpaHLNnVyL6qTqn0zh_S81mXeqZmdXozlhrjnF3wCUdz0rXDJqmBnq50fA"

func main() {
	url := "https://deck-api.ngcloud.ru/api/v1/index.cfm/instances"
	payload := map[string]interface{}{
		"serviceId":   1,
		"displayName": "DebugReq-Test",
		"descr":       "Created via debug tool",
	}
	b, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Connection", "close") // Try to force close

	client := &http.Client{Timeout: 10 * time.Second}
	fmt.Println("Sending request...")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %s\n", resp.Status)
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Body: %s\n", string(body))
}
