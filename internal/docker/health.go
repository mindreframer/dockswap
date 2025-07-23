package docker

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"dockswap/internal/config"
)

type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusStarting  HealthStatus = "starting"
	HealthStatusUnknown   HealthStatus = "unknown"
)

type HealthCheckResult struct {
	Status        HealthStatus
	DockerHealth  HealthStatus
	HTTPHealth    HealthStatus
	Message       string
	LastCheck     time.Time
	CheckDuration time.Duration
}

type HTTPHealthChecker struct {
	client *http.Client
}

func NewHTTPHealthChecker(timeout time.Duration) *HTTPHealthChecker {
	return &HTTPHealthChecker{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (dm *DockerManager) CheckContainerHealth(ctx context.Context, appName, color string, appConfig *config.AppConfig) (*HealthCheckResult, error) {
	startTime := time.Now()
	result := &HealthCheckResult{
		Status:    HealthStatusUnknown,
		LastCheck: startTime,
	}

	// Get container info
	containerInfo, err := dm.GetContainerInfo(ctx, appName, color)
	if err != nil {
		result.Message = fmt.Sprintf("Failed to get container info: %v", err)
		result.CheckDuration = time.Since(startTime)
		return result, nil
	}

	// Check Docker health status
	dockerHealth := parseDockerHealthStatus(containerInfo.Health)
	result.DockerHealth = dockerHealth

	// If container is not running, it's unhealthy
	if containerInfo.State != "running" {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("Container is not running (state: %s)", containerInfo.State)
		result.CheckDuration = time.Since(startTime)
		return result, nil
	}

	// Check HTTP health if configured
	if appConfig.HealthCheck.Endpoint != "" {
		httpHealth, httpErr := dm.checkHTTPHealth(appName, color, appConfig)
		result.HTTPHealth = httpHealth

		if httpErr != nil {
			result.Status = HealthStatusUnhealthy
			result.Message = fmt.Sprintf("HTTP health check failed: %v", httpErr)
			result.CheckDuration = time.Since(startTime)
			return result, nil
		}
	}

	// Determine overall health status
	result.Status = dm.determineOverallHealth(result.DockerHealth, result.HTTPHealth, appConfig)

	if result.Status == HealthStatusHealthy {
		result.Message = "All health checks passed"
	} else if result.Status == HealthStatusStarting {
		result.Message = "Container is starting up"
	} else {
		result.Message = "Health checks failed"
	}

	result.CheckDuration = time.Since(startTime)
	return result, nil
}

func (dm *DockerManager) checkHTTPHealth(appName, color string, appConfig *config.AppConfig) (HealthStatus, error) {
	// Determine the port to check
	var port int
	if color == "blue" {
		port = appConfig.Ports.Blue
	} else {
		port = appConfig.Ports.Green
	}

	// Build health check URL
	url := fmt.Sprintf("http://localhost:%d%s", port, appConfig.HealthCheck.Endpoint)

	// Create HTTP health checker with configured timeout
	checker := NewHTTPHealthChecker(appConfig.HealthCheck.Timeout)

	// Perform health check with retries
	for attempt := 0; attempt < appConfig.HealthCheck.Retries; attempt++ {
		if attempt > 0 {
			time.Sleep(appConfig.HealthCheck.Interval)
		}

		req, err := http.NewRequest(appConfig.HealthCheck.Method, url, nil)
		if err != nil {
			continue
		}

		resp, err := checker.client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == appConfig.HealthCheck.ExpectedStatus {
			return HealthStatusHealthy, nil
		}
	}

	return HealthStatusUnhealthy, fmt.Errorf("HTTP health check failed after %d attempts", appConfig.HealthCheck.Retries)
}

func (dm *DockerManager) WaitForHealthy(ctx context.Context, appName, color string, appConfig *config.AppConfig, timeout time.Duration) error {
	startTime := time.Now()
	ticker := time.NewTicker(appConfig.HealthCheck.Interval)
	defer ticker.Stop()

	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	successCount := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeoutTimer.C:
			return fmt.Errorf("health check timeout after %v", timeout)
		case <-ticker.C:
			result, err := dm.CheckContainerHealth(ctx, appName, color, appConfig)
			if err != nil {
				return fmt.Errorf("health check error: %w", err)
			}

			if result.Status == HealthStatusHealthy {
				successCount++
				if successCount >= appConfig.HealthCheck.SuccessThreshold {
					return nil // Health check passed
				}
			} else {
				successCount = 0 // Reset on failure
			}

			// Log progress (in real implementation, you'd use a proper logger)
			elapsed := time.Since(startTime)
			fmt.Printf("Health check %s-%s: %s (attempt %d/%d, elapsed: %v)\n",
				appName, color, result.Status, successCount, appConfig.HealthCheck.SuccessThreshold, elapsed)
		}
	}
}

func parseDockerHealthStatus(healthStr string) HealthStatus {
	switch strings.ToLower(healthStr) {
	case "healthy":
		return HealthStatusHealthy
	case "unhealthy":
		return HealthStatusUnhealthy
	case "starting":
		return HealthStatusStarting
	default:
		return HealthStatusUnknown
	}
}

func (dm *DockerManager) determineOverallHealth(dockerHealth, httpHealth HealthStatus, appConfig *config.AppConfig) HealthStatus {
	// If HTTP health check is not configured, rely on Docker health
	if appConfig.HealthCheck.Endpoint == "" {
		if dockerHealth == HealthStatusUnknown {
			// If no Docker health check is configured, assume healthy if container is running
			return HealthStatusHealthy
		}
		return dockerHealth
	}

	// Both Docker and HTTP health checks are configured
	if dockerHealth == HealthStatusUnhealthy || httpHealth == HealthStatusUnhealthy {
		return HealthStatusUnhealthy
	}

	if dockerHealth == HealthStatusStarting || httpHealth == HealthStatusStarting {
		return HealthStatusStarting
	}

	if dockerHealth == HealthStatusHealthy && httpHealth == HealthStatusHealthy {
		return HealthStatusHealthy
	}

	// If we have mixed results or unknown status, consider it as starting
	return HealthStatusStarting
}

func (dm *DockerManager) IsContainerHealthy(ctx context.Context, appName, color string, appConfig *config.AppConfig) (bool, error) {
	result, err := dm.CheckContainerHealth(ctx, appName, color, appConfig)
	if err != nil {
		return false, err
	}

	return result.Status == HealthStatusHealthy, nil
}
