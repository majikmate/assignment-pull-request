package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// OCIImageInfo represents the OCI image metadata
type OCIImageInfo struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	Version       string `json:"version"`
	Revision      string `json:"revision"`
	RefName       string `json:"ref_name"`
	Source        string `json:"source"`
	URL           string `json:"url"`
	Documentation string `json:"documentation"`
	Created       string `json:"created"`
	Authors       string `json:"authors"`
	Vendor        string `json:"vendor"`
	Licenses      string `json:"licenses"`
}

func main() {
	// Try to get container metadata from labels first
	info, err := getContainerMetadata()
	if err != nil {
		fmt.Printf("Warning: Could not read container labels, falling back to environment variables: %v\n", err)
		// Fallback to environment variables
		info = getMetadataFromEnv()
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	// Ensure .devcontainer directory exists
	devcontainerDir := ".devcontainer"
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating .devcontainer directory: %v\n", err)
		os.Exit(1)
	}

	// Write to info.json file
	outputPath := filepath.Join(devcontainerDir, "info.json")
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing to %s: %v\n", outputPath, err)
		os.Exit(1)
	}

	fmt.Printf("Container metadata saved to %s\n", outputPath)

	// Also print the metadata to stdout for verification
	fmt.Println("\nContainer Metadata:")
	fmt.Println(string(jsonData))
}

// getContainerMetadata reads metadata from container labels
func getContainerMetadata() (OCIImageInfo, error) {
	// Get current container ID
	containerID, err := getCurrentContainerID()
	if err != nil {
		return OCIImageInfo{}, fmt.Errorf("failed to get container ID: %w", err)
	}

	// Inspect container to get labels
	cmd := exec.Command("docker", "inspect", "--format", "{{json .Config.Labels}}", containerID)
	output, err := cmd.Output()
	if err != nil {
		return OCIImageInfo{}, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Parse labels JSON
	var labels map[string]string
	if err := json.Unmarshal(output, &labels); err != nil {
		return OCIImageInfo{}, fmt.Errorf("failed to parse labels JSON: %w", err)
	}

	// Extract OCI metadata from labels
	info := OCIImageInfo{
		Title:         getLabel(labels, "org.opencontainers.image.title"),
		Description:   getLabel(labels, "org.opencontainers.image.description"),
		Version:       getLabel(labels, "org.opencontainers.image.version"),
		Revision:      getLabel(labels, "org.opencontainers.image.revision"),
		RefName:       getLabel(labels, "org.opencontainers.image.ref.name"),
		Source:        getLabel(labels, "org.opencontainers.image.source"),
		URL:           getLabel(labels, "org.opencontainers.image.url"),
		Documentation: getLabel(labels, "org.opencontainers.image.documentation"),
		Created:       getLabel(labels, "org.opencontainers.image.created"),
		Authors:       getLabel(labels, "org.opencontainers.image.authors"),
		Vendor:        getLabel(labels, "org.opencontainers.image.vendor"),
		Licenses:      getLabel(labels, "org.opencontainers.image.licenses"),
	}

	return info, nil
}

// getCurrentContainerID tries to determine the current container ID
func getCurrentContainerID() (string, error) {
	// Method 1: Try reading from /proc/self/cgroup (works in most containers)
	if data, err := os.ReadFile("/proc/self/cgroup"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.Contains(line, "docker") {
				parts := strings.Split(line, "/")
				if len(parts) > 0 {
					containerID := parts[len(parts)-1]
					if len(containerID) >= 12 {
						return containerID, nil
					}
				}
			}
		}
	}

	// Method 2: Try reading from hostname (works if hostname is container ID)
	if hostname, err := os.Hostname(); err == nil && len(hostname) >= 12 {
		return hostname, nil
	}

	// Method 3: Try environment variables that might contain container ID
	if containerID := os.Getenv("HOSTNAME"); containerID != "" && len(containerID) >= 12 {
		return containerID, nil
	}

	return "", fmt.Errorf("could not determine container ID")
}

// getLabel safely gets a label value from the labels map
func getLabel(labels map[string]string, key string) string {
	if value, exists := labels[key]; exists {
		return value
	}
	return ""
}

// getMetadataFromEnv reads metadata from environment variables (fallback)
func getMetadataFromEnv() OCIImageInfo {
	return OCIImageInfo{
		Title:         getEnvWithDefault("OCI_IMAGE_TITLE", ""),
		Description:   getEnvWithDefault("OCI_IMAGE_DESCRIPTION", ""),
		Version:       getEnvWithDefault("OCI_IMAGE_VERSION", ""),
		Revision:      getEnvWithDefault("OCI_IMAGE_REVISION", ""),
		RefName:       getEnvWithDefault("OCI_IMAGE_REF_NAME", ""),
		Source:        getEnvWithDefault("OCI_IMAGE_SOURCE", ""),
		URL:           getEnvWithDefault("OCI_IMAGE_URL", ""),
		Documentation: getEnvWithDefault("OCI_IMAGE_DOCUMENTATION", ""),
		Created:       getEnvWithDefault("OCI_IMAGE_CREATED", time.Now().UTC().Format(time.RFC3339)),
		Authors:       getEnvWithDefault("OCI_IMAGE_AUTHORS", ""),
		Vendor:        getEnvWithDefault("OCI_IMAGE_VENDOR", ""),
		Licenses:      getEnvWithDefault("OCI_IMAGE_LICENSES", ""),
	}
}

// getEnvWithDefault returns the value of the environment variable or a default value
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
