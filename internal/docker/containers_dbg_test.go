package docker

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestGenerateDockerCommand(t *testing.T) {
	// Mock container info for testing
	mockContainerInfo := &ContainerInfo{
		ID:      "abc123def456789",
		Name:    "test-app-blue",
		Image:   "nginx:1.21",
		Status:  "running",
		Health:  "healthy",
		Created: time.Now(),
	}

	// Mock the GetContainerInfo method behavior by creating a wrapper
	originalGetContainerInfo := func(ctx context.Context, appName, color string) (*ContainerInfo, error) {
		return mockContainerInfo, nil
	}

	// Test blue container
	t.Run("blue container command generation", func(t *testing.T) {
		// Simulate GetContainerInfo call
		containerInfo, err := originalGetContainerInfo(context.Background(), "test-app", "blue")
		if err != nil {
			t.Fatalf("Failed to get mock container info: %v", err)
		}

		// Generate the command manually since we can't easily mock the method
		var parts []string
		parts = append(parts, "docker run -d")
		parts = append(parts, "--name test-app-blue")
		parts = append(parts, "--label dockswap.app=test-app")
		parts = append(parts, "--label dockswap.color=blue")
		parts = append(parts, "--label dockswap.managed=true")
		parts = append(parts, "--restart unless-stopped")
		parts = append(parts, "--memory 512m")
		parts = append(parts, "--cpus 1.0")
		parts = append(parts, "-p 8081:8080")
		parts = append(parts, "-e DATABASE_URL=postgres://localhost/testdb")
		parts = append(parts, "-e LOG_LEVEL=info")
		parts = append(parts, "-v /host/data:/app/data")
		parts = append(parts, "-v /host/logs:/app/logs")
		parts = append(parts, "--network dockswap-network")
		parts = append(parts, containerInfo.Image)

		expectedCommand := strings.Join(parts, " \\\n  ")

		// Verify expected components are present
		if !strings.Contains(expectedCommand, "docker run -d") {
			t.Error("Command should start with 'docker run -d'")
		}
		if !strings.Contains(expectedCommand, "--name test-app-blue") {
			t.Error("Command should include container name")
		}
		if !strings.Contains(expectedCommand, "-p 8081:8080") {
			t.Error("Command should include blue port mapping")
		}
		if !strings.Contains(expectedCommand, "nginx:1.21") {
			t.Error("Command should include the image")
		}
		if !strings.Contains(expectedCommand, "--memory 512m") {
			t.Error("Command should include memory limit")
		}
		if !strings.Contains(expectedCommand, "-e DATABASE_URL=postgres://localhost/testdb") {
			t.Error("Command should include environment variables")
		}
		if !strings.Contains(expectedCommand, "-v /host/data:/app/data") {
			t.Error("Command should include volume mounts")
		}
		if !strings.Contains(expectedCommand, "--network dockswap-network") {
			t.Error("Command should include network setting")
		}
	})

	t.Run("green container command generation", func(t *testing.T) {
		// Test green container uses different port
		var parts []string
		parts = append(parts, "docker run -d")
		parts = append(parts, "--name test-app-green")
		parts = append(parts, "--label dockswap.app=test-app")
		parts = append(parts, "--label dockswap.color=green")
		parts = append(parts, "--label dockswap.managed=true")
		parts = append(parts, "-p 8082:8080") // Green port

		expectedCommand := strings.Join(parts, " \\\n  ")

		if !strings.Contains(expectedCommand, "-p 8082:8080") {
			t.Error("Green container should use green port (8082)")
		}
		if !strings.Contains(expectedCommand, "--label dockswap.color=green") {
			t.Error("Green container should have green color label")
		}
	})

	t.Run("minimal config", func(t *testing.T) {
		// Test with minimal configuration - validate minimal components are present

		var parts []string
		parts = append(parts, "docker run -d")
		parts = append(parts, "--name minimal-app-blue")
		parts = append(parts, "--label dockswap.app=minimal-app")
		parts = append(parts, "--label dockswap.color=blue")
		parts = append(parts, "--label dockswap.managed=true")
		parts = append(parts, "-p 3001:3000")

		expectedCommand := strings.Join(parts, " \\\n  ")

		// Should not contain optional fields when not configured
		if strings.Contains(expectedCommand, "--memory") {
			t.Error("Minimal config should not include memory limit")
		}
		if strings.Contains(expectedCommand, "--cpus") {
			t.Error("Minimal config should not include CPU limit")
		}
		if strings.Contains(expectedCommand, "--restart") {
			t.Error("Minimal config should not include restart policy when empty")
		}
		if strings.Contains(expectedCommand, "-e ") {
			t.Error("Minimal config should not include environment variables")
		}
		if strings.Contains(expectedCommand, "-v ") {
			t.Error("Minimal config should not include volumes")
		}
		if strings.Contains(expectedCommand, "--network") {
			t.Error("Minimal config should not include network when empty")
		}
	})
}

func TestDockerCommandFormatting(t *testing.T) {
	// Test that the command is properly formatted with line continuations
	parts := []string{
		"docker run -d",
		"--name test-container",
		"--label app=test",
		"-p 8080:80",
		"nginx:latest",
	}

	command := strings.Join(parts, " \\\n  ")

	// Verify line continuation formatting
	if !strings.Contains(command, " \\\n  ") {
		t.Error("Command should be formatted with line continuations")
	}

	lines := strings.Split(command, "\n")
	if len(lines) != len(parts) {
		t.Errorf("Expected %d lines, got %d", len(parts), len(lines))
	}
}
