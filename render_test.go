package main

import (
	"os"
	"strings"
	"testing"
)

func TestRenderYamlExists(t *testing.T) {
	data, err := os.ReadFile("render.yaml")
	if err != nil {
		t.Fatalf("reading render.yaml: %v", err)
	}
	if len(data) == 0 {
		t.Error("render.yaml is empty")
	}
}

func TestRenderYamlStartCommand(t *testing.T) {
	data, err := os.ReadFile("render.yaml")
	if err != nil {
		t.Fatalf("reading render.yaml: %v", err)
	}
	content := string(data)

	// Must contain ./battle-bracket-wheels
	if !strings.Contains(content, "./battle-bracket-wheels") {
		t.Error("render.yaml missing ./battle-bracket-wheels in startCommand")
	}

	// Must NOT contain go run
	if strings.Contains(content, "go run") {
		t.Error("render.yaml contains 'go run' — should use compiled binary")
	}
}

func TestRenderYamlPort(t *testing.T) {
	data, err := os.ReadFile("render.yaml")
	if err != nil {
		t.Fatalf("reading render.yaml: %v", err)
	}
	content := string(data)

	// Must reference PORT as an environment variable
	if !strings.Contains(content, "PORT") {
		t.Error("render.yaml missing PORT env var")
	}
}

func TestRenderYamlServiceType(t *testing.T) {
	data, err := os.ReadFile("render.yaml")
	if err != nil {
		t.Fatalf("reading render.yaml: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "type: web") {
		t.Error("render.yaml missing 'type: web'")
	}
}
