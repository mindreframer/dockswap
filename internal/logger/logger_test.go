package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		level    int
		expected int
	}{
		{"Valid Error Level", LevelError, LevelError},
		{"Valid Info Level", LevelInfo, LevelInfo},
		{"Valid Debug Level", LevelDebug, LevelDebug},
		{"Invalid Low Level", 0, LevelInfo},
		{"Invalid High Level", 4, LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New(tt.level)
			if logger.GetLevel() != tt.expected {
				t.Errorf("New(%d) = %d, want %d", tt.level, logger.GetLevel(), tt.expected)
			}
		})
	}
}

func TestSetLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOutput(LevelInfo, &buf).(*StandardLogger)

	// Test valid level changes
	logger.SetLevel(LevelDebug)
	if logger.GetLevel() != LevelDebug {
		t.Errorf("SetLevel(LevelDebug) failed, got %d", logger.GetLevel())
	}

	logger.SetLevel(LevelError)
	if logger.GetLevel() != LevelError {
		t.Errorf("SetLevel(LevelError) failed, got %d", logger.GetLevel())
	}

	// Test invalid level changes (should be ignored)
	originalLevel := logger.GetLevel()
	logger.SetLevel(0)
	if logger.GetLevel() != originalLevel {
		t.Errorf("SetLevel(0) should be ignored, but level changed")
	}

	logger.SetLevel(4)
	if logger.GetLevel() != originalLevel {
		t.Errorf("SetLevel(4) should be ignored, but level changed")
	}
}

func TestLogLevelFiltering(t *testing.T) {
	tests := []struct {
		name         string
		loggerLevel  int
		messageLevel string
		shouldLog    bool
	}{
		{"Error level logs error", LevelError, "Error", true},
		{"Error level skips info", LevelError, "Info", false},
		{"Error level skips debug", LevelError, "Debug", false},
		{"Info level logs error", LevelInfo, "Error", true},
		{"Info level logs info", LevelInfo, "Info", true},
		{"Info level skips debug", LevelInfo, "Debug", false},
		{"Debug level logs error", LevelDebug, "Error", true},
		{"Debug level logs info", LevelDebug, "Info", true},
		{"Debug level logs debug", LevelDebug, "Debug", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewWithOutput(tt.loggerLevel, &buf)

			// Call the appropriate logging method
			switch tt.messageLevel {
			case "Error":
				logger.Error("test error message")
			case "Info":
				logger.Info("test info message")
			case "Debug":
				logger.Debug("test debug message")
			}

			output := buf.String()
			hasOutput := len(output) > 0

			if hasOutput != tt.shouldLog {
				t.Errorf("Level %d, Message %s: expected shouldLog=%v, got hasOutput=%v",
					tt.loggerLevel, tt.messageLevel, tt.shouldLog, hasOutput)
			}

			if tt.shouldLog {
				// Verify the output contains expected components
				expectedLevelInOutput := strings.ToUpper(tt.messageLevel)
				if !strings.Contains(output, expectedLevelInOutput) {
					t.Errorf("Output should contain level '%s', got: %s", expectedLevelInOutput, output)
				}
				if !strings.Contains(output, "test "+strings.ToLower(tt.messageLevel)+" message") {
					t.Errorf("Output should contain message, got: %s", output)
				}
			}
		})
	}
}

func TestLogFormatting(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOutput(LevelDebug, &buf)

	logger.Info("test message with %s and %d", "string", 42)
	output := buf.String()

	// Check that the message was formatted correctly
	if !strings.Contains(output, "test message with string and 42") {
		t.Errorf("Expected formatted message, got: %s", output)
	}

	// Check that it contains timestamp, level, and message
	if !strings.Contains(output, "INFO") {
		t.Errorf("Output should contain 'INFO', got: %s", output)
	}

	// Check that it has timestamp format (YYYY-MM-DD HH:MM:SS)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 1 {
		t.Fatalf("Expected at least one line of output")
	}

	// The format should be: [YYYY-MM-DD HH:MM:SS] LEVEL: message
	if !strings.Contains(lines[0], "[") || !strings.Contains(lines[0], "]") {
		t.Errorf("Output should contain timestamp in brackets, got: %s", lines[0])
	}
}

func TestConcurrentAccess(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOutput(LevelDebug, &buf)

	// Test concurrent level changes and logging
	done := make(chan bool, 20)

	// Start multiple goroutines that change log level
	for i := 0; i < 10; i++ {
		go func(id int) {
			logger.SetLevel(LevelError + (id % 3))
			done <- true
		}(i)
	}

	// Start multiple goroutines that log messages
	for i := 0; i < 10; i++ {
		go func(id int) {
			logger.Info("concurrent message %d", id)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 20; i++ {
		<-done
	}

	// Test should complete without panic
	// The exact output is non-deterministic due to concurrency,
	// but we can check that we got some output
	output := buf.String()
	if len(output) == 0 {
		t.Error("Expected some output from concurrent logging")
	}
}

func TestLogLevels(t *testing.T) {
	// Test that the level constants have expected values
	if LevelError != 1 {
		t.Errorf("LevelError should be 1, got %d", LevelError)
	}
	if LevelInfo != 2 {
		t.Errorf("LevelInfo should be 2, got %d", LevelInfo)
	}
	if LevelDebug != 3 {
		t.Errorf("LevelDebug should be 3, got %d", LevelDebug)
	}
}

func TestMultipleMessages(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOutput(LevelDebug, &buf)

	logger.Error("error 1")
	logger.Info("info 1")
	logger.Debug("debug 1")
	logger.Error("error 2")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have 4 lines of output
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines of output, got %d: %v", len(lines), lines)
	}

	// Check each line contains the expected content
	expectedContents := []string{"error 1", "info 1", "debug 1", "error 2"}
	expectedLevels := []string{"ERROR", "INFO", "DEBUG", "ERROR"}

	for i, line := range lines {
		if !strings.Contains(line, expectedContents[i]) {
			t.Errorf("Line %d should contain '%s', got: %s", i, expectedContents[i], line)
		}
		if !strings.Contains(line, expectedLevels[i]) {
			t.Errorf("Line %d should contain '%s', got: %s", i, expectedLevels[i], line)
		}
	}
}
