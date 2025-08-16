package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

var (
	serverURL = flag.String("server", "http://localhost:8080", "API server URL")
)

func main() {
	flag.Parse()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "create":
		if len(os.Args) < 3 {
			fmt.Println("Usage: cli create -f <filename>")
			os.Exit(1)
		}
		createResource()
	case "get":
		if len(os.Args) < 3 {
			fmt.Println("Usage: cli get <resource> [name]")
			os.Exit(1)
		}
		getResource()
	case "delete":
		if len(os.Args) < 4 {
			fmt.Println("Usage: cli delete <resource> <name>")
			os.Exit(1)
		}
		deleteResource()
	case "watch":
		if len(os.Args) < 4 {
			fmt.Println("Usage: cli watch <resource> <name>")
			os.Exit(1)
		}
		watchResource()
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Minik8s CLI")
	fmt.Println("Usage:")
	fmt.Println("  cli create -f <filename>     Create a resource from file")
	fmt.Println("  cli get <resource> [name]    Get resources")
	fmt.Println("  cli delete <resource> <name> Delete a resource")
	fmt.Println("  cli watch <resource> <name>  Watch a resource")
	fmt.Println("")
	fmt.Println("Resources: pods, nodes")
	fmt.Println("Examples:")
	fmt.Println("  cli create -f pod.yaml")
	fmt.Println("  cli get pods")
	fmt.Println("  cli get pods my-pod")
	fmt.Println("  cli delete pods my-pod")
}

func createResource() {
	var filename string
	for i, arg := range os.Args {
		if arg == "-f" && i+1 < len(os.Args) {
			filename = os.Args[i+1]
			break
		}
	}

	if filename == "" {
		fmt.Println("Error: -f flag is required")
		os.Exit(1)
	}

	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Parse YAML/JSON
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		fmt.Printf("Error parsing file: %v\n", err)
		os.Exit(1)
	}

	kind, ok := obj["kind"].(string)
	if !ok {
		fmt.Println("Error: kind field is required")
		os.Exit(1)
	}

	// Determine endpoint based on kind
	var endpoint string
	switch strings.ToLower(kind) {
	case "pod":
		namespace := getNamespace(obj, "default")
		endpoint = fmt.Sprintf("%s/api/v1alpha1/namespaces/%s/pods", *serverURL, namespace)
	case "node":
		endpoint = fmt.Sprintf("%s/api/v1alpha1/nodes", *serverURL)
	default:
		fmt.Printf("Error: unsupported resource kind: %s\n", kind)
		os.Exit(1)
	}

	// Send request
	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(data))
	if err != nil {
		fmt.Printf("Error creating resource: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		fmt.Printf("Successfully created %s\n", kind)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error creating resource: %s - %s\n", resp.Status, string(body))
		os.Exit(1)
	}
}

func getResource() {
	resource := os.Args[2]
	var name string
	if len(os.Args) > 3 {
		name = os.Args[3]
	}

	var endpoint string
	switch strings.ToLower(resource) {
	case "pods":
		if name != "" {
			// Get specific pod
			endpoint = fmt.Sprintf("%s/api/v1alpha1/namespaces/default/pods/%s", *serverURL, name)
		} else {
			// List all pods
			endpoint = fmt.Sprintf("%s/api/v1alpha1/pods", *serverURL)
		}
	case "nodes":
		if name != "" {
			// Get specific node
			endpoint = fmt.Sprintf("%s/api/v1alpha1/nodes/%s", *serverURL, name)
		} else {
			// List all nodes
			endpoint = fmt.Sprintf("%s/api/v1alpha1/nodes", *serverURL)
		}
	default:
		fmt.Printf("Error: unsupported resource: %s\n", resource)
		os.Exit(1)
	}

	// Send request
	resp, err := http.Get(endpoint)
	if err != nil {
		fmt.Printf("Error getting resource: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Println(string(body))
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error getting resource: %s - %s\n", resp.Status, string(body))
		os.Exit(1)
	}
}

func deleteResource() {
	resource := os.Args[2]
	name := os.Args[3]

	var endpoint string
	switch strings.ToLower(resource) {
	case "pods":
		endpoint = fmt.Sprintf("%s/api/v1alpha1/namespaces/default/pods/%s", *serverURL, name)
	case "nodes":
		endpoint = fmt.Sprintf("%s/api/v1alpha1/nodes/%s", *serverURL, name)
	default:
		fmt.Printf("Error: unsupported resource: %s\n", resource)
		os.Exit(1)
	}

	// Send request
	req, err := http.NewRequest("DELETE", endpoint, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		os.Exit(1)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error deleting resource: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Printf("Successfully deleted %s %s\n", resource, name)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error deleting resource: %s - %s\n", resp.Status, string(body))
		os.Exit(1)
	}
}

func watchResource() {
	resource := os.Args[2]
	name := os.Args[3]

	var endpoint string
	switch strings.ToLower(resource) {
	case "pods":
		endpoint = fmt.Sprintf("%s/api/v1alpha1/namespaces/default/pods/%s/watch", *serverURL, name)
	case "nodes":
		endpoint = fmt.Sprintf("%s/api/v1alpha1/nodes/%s/watch", *serverURL, name)
	default:
		fmt.Printf("Error: unsupported resource: %s\n", resource)
		os.Exit(1)
	}

	// Send request
	resp, err := http.Get(endpoint)
	if err != nil {
		fmt.Printf("Error watching resource: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error watching resource: %s - %s\n", resp.Status, string(body))
		os.Exit(1)
	}

	fmt.Printf("Watching %s %s... (Press Ctrl+C to stop)\n", resource, name)

	// Stream events
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Printf("Error reading event: %v\n", err)
			break
		}

		line = strings.TrimSpace(line)
		if line != "" {
			fmt.Printf("Event: %s\n", line)
		}
	}
}

func getNamespace(obj map[string]interface{}, defaultNS string) string {
	if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
		if namespace, ok := metadata["namespace"].(string); ok && namespace != "" {
			return namespace
		}
	}
	return defaultNS
}
