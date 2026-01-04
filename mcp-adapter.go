package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// This is a lightweight MCP adapter that forwards requests to the Wails bridge HTTP endpoint
// The Wails app runs the actual bridge with Ngrok tunnel
// This adapter speaks MCP stdio protocol and forwards to HTTP

func main() {
	// Read config to get tunnel URL
	configPath := "bridge-config.json"
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		os.Exit(1)
	}

	var config map[string]interface{}
	json.Unmarshal(data, &config)

	// Start MCP stdio server
	reader := bufio.NewReader(os.Stdin)
	
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse JSON-RPC request
		var request map[string]interface{}
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			continue
		}

		method, _ := request["method"].(string)
		id := request["id"]

		switch method {
		case "initialize":
			// Respond with server info
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"serverInfo": map[string]interface{}{
						"name":    "Remote Bridge (Wails)",
						"version": "2.0.0",
					},
					"capabilities": map[string]interface{}{
						"tools": map[string]interface{}{},
					},
				},
			}
			sendResponse(response)

		case "tools/list":
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "ask_remote_human",
							"description": "Ask the user a question via configured notification channels",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"question": map[string]interface{}{
										"type":        "string",
										"description": "The question to ask",
									},
									"options": map[string]interface{}{
										"type":        "array",
										"description": "Available response options",
										"items":       map[string]interface{}{"type": "string"},
									},
								},
								"required": []string{"question", "options"},
							},
						},
					},
				},
			}
			sendResponse(response)

		case "tools/call":
			// Forward to Wails bridge HTTP endpoint
			params, _ := request["params"].(map[string]interface{})
			toolName, _ := params["name"].(string)
			args, _ := params["arguments"].(map[string]interface{})

			if toolName == "ask_remote_human" {
				result := callWailsBridge(args)
				response := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      id,
					"result": map[string]interface{}{
						"content": []map[string]interface{}{
							{"type": "text", "text": result},
						},
					},
				}
				sendResponse(response)
			}

		case "notifications/initialized":
			// Ignore notifications
		}
	}
}

func sendResponse(response map[string]interface{}) {
	data, _ := json.Marshal(response)
	fmt.Println(string(data))
}

func callWailsBridge(args map[string]interface{}) string {
	// Read tunnel URL from file - check multiple locations
	paths := []string{
		"tunnel-url.txt",
		"bridge-ui/tunnel-url.txt",
		"../tunnel-url.txt",
	}
	
	var tunnelURL string
	for _, p := range paths {
		data, err := ioutil.ReadFile(p)
		if err == nil {
			tunnelURL = strings.TrimSpace(string(data))
			break
		}
	}
	
	if tunnelURL == "" {
		return "Error: Bridge not running. Please start the Remote Bridge app first."
	}

	// Send notification request
	question, _ := args["question"].(string)
	options, _ := args["options"].([]interface{})

	requestBody := map[string]interface{}{
		"question": question,
		"options":  options,
	}
	bodyData, _ := json.Marshal(requestBody)

	resp, err := http.Post(tunnelURL+"/ask", "application/json", bytes.NewReader(bodyData))
	if err != nil {
		return fmt.Sprintf("Error connecting to bridge: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)
	
	if answer, ok := result["answer"].(string); ok {
		return answer
	}
	
	return string(respBody)
}
