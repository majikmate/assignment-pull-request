package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	// Get container metadata from environment variables
	info := getMetadataFromEnv()

	// Don't write info.json if Version is empty
	if info.Version == "" {
		fmt.Println("No OCI metadata found")
		return
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

// getMetadataFromEnv reads metadata from environment variables
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
