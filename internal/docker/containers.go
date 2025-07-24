package docker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"

	"dockswap/internal/config"
)

type ContainerInfo struct {
	ID      string
	Name    string
	Image   string
	Status  string
	State   string
	Health  string
	Ports   map[int]int // container:host port mapping
	Created time.Time
}

func (dm *DockerManager) CreateContainer(ctx context.Context, appName, color, image string, appConfig *config.AppConfig) (*ContainerInfo, error) {
	containerName := fmt.Sprintf("%s-%s", appName, color)

	// Build container configuration
	containerConfig := &container.Config{
		Image: image,
		Env:   buildEnvironmentVars(appConfig.Docker.Environment),
		Labels: map[string]string{
			"dockswap.app":     appName,
			"dockswap.color":   color,
			"dockswap.managed": "true",
		},
	}

	// Build host configuration
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyMode(appConfig.Docker.RestartPolicy),
		},
		AutoRemove: false,
	}

	// Apply resource limits
	if err := applyResourceLimits(hostConfig, appConfig); err != nil {
		return nil, fmt.Errorf("failed to apply resource limits: %w", err)
	}

	// Apply port mappings
	if err := applyPortMappings(hostConfig, containerConfig, appConfig, color); err != nil {
		return nil, fmt.Errorf("failed to apply port mappings: %w", err)
	}

	// Apply volume mounts
	if err := applyVolumeMounts(hostConfig, appConfig); err != nil {
		return nil, fmt.Errorf("failed to apply volume mounts: %w", err)
	}

	// Network configuration
	networkingConfig := &network.NetworkingConfig{}
	if appConfig.Docker.Network != "" {
		networkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			appConfig.Docker.Network: {},
		}
	}

	// Create container
	resp, err := dm.client.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to create container %s: %w", containerName, err)
	}

	return &ContainerInfo{
		ID:      resp.ID,
		Name:    containerName,
		Image:   image,
		Status:  "created",
		State:   "created",
		Health:  "unknown",
		Created: time.Now(),
	}, nil
}

func (dm *DockerManager) StartContainer(ctx context.Context, containerID string) error {
	err := dm.client.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container %s: %w", containerID, err)
	}
	return nil
}

func (dm *DockerManager) StopContainer(ctx context.Context, appName, color string, timeout time.Duration) error {
	containerName := fmt.Sprintf("%s-%s", appName, color)

	// Find container
	containers, err := dm.findContainers(ctx, appName, color)
	if err != nil {
		return fmt.Errorf("failed to find container %s: %w", containerName, err)
	}

	if len(containers) == 0 {
		return fmt.Errorf("container %s not found", containerName)
	}

	// Stop container with timeout
	timeoutSeconds := int(timeout.Seconds())
	stopOptions := container.StopOptions{
		Timeout: &timeoutSeconds,
	}

	err = dm.client.ContainerStop(ctx, containers[0].ID, stopOptions)
	if err != nil {
		return fmt.Errorf("failed to stop container %s: %w", containerName, err)
	}

	return nil
}

func (dm *DockerManager) RemoveContainer(ctx context.Context, appName, color string, force bool) error {
	containerName := fmt.Sprintf("%s-%s", appName, color)

	// Find container
	containers, err := dm.findContainers(ctx, appName, color)
	if err != nil {
		return fmt.Errorf("failed to find container %s: %w", containerName, err)
	}

	if len(containers) == 0 {
		return fmt.Errorf("container %s not found", containerName)
	}

	// Remove container
	removeOptions := container.RemoveOptions{
		Force:         force,
		RemoveVolumes: false, // Keep volumes for data persistence
	}

	err = dm.client.ContainerRemove(ctx, containers[0].ID, removeOptions)
	if err != nil {
		return fmt.Errorf("failed to remove container %s: %w", containerName, err)
	}

	return nil
}

func (dm *DockerManager) GetContainerInfo(ctx context.Context, appName, color string) (*ContainerInfo, error) {
	containers, err := dm.findContainers(ctx, appName, color)
	if err != nil {
		return nil, fmt.Errorf("failed to find container: %w", err)
	}

	if len(containers) == 0 {
		return nil, fmt.Errorf("container %s-%s not found", appName, color)
	}

	containerJSON, err := dm.client.ContainerInspect(ctx, containers[0].ID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Build port mapping
	ports := make(map[int]int)
	for containerPort, bindings := range containerJSON.NetworkSettings.Ports {
		if len(bindings) > 0 {
			containerPortInt := containerPort.Int()
			for _, binding := range bindings {
				if binding.HostPort != "" {
					// Parse host port
					// This is simplified - in reality you'd parse the port string
					ports[containerPortInt] = containerPortInt
				}
			}
		}
	}

	// Determine health status
	health := "unknown"
	if containerJSON.State.Health != nil {
		health = strings.ToLower(containerJSON.State.Health.Status)
	}

	return &ContainerInfo{
		ID:      containerJSON.ID,
		Name:    strings.TrimPrefix(containerJSON.Name, "/"),
		Image:   containerJSON.Config.Image,
		Status:  containerJSON.State.Status,
		State:   containerJSON.State.Status,
		Health:  health,
		Ports:   ports,
		Created: parseCreatedTime(containerJSON.Created),
	}, nil
}

func (dm *DockerManager) ListAppContainers(ctx context.Context, appName string) ([]*ContainerInfo, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("dockswap.app=%s", appName))
	filterArgs.Add("label", "dockswap.managed=true")

	containers, err := dm.client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers for app %s: %w", appName, err)
	}

	var result []*ContainerInfo
	for _, container := range containers {
		info := &ContainerInfo{
			ID:     container.ID,
			Image:  container.Image,
			Status: container.Status,
			State:  container.State,
			Health: "unknown",
		}

		if len(container.Names) > 0 {
			info.Name = strings.TrimPrefix(container.Names[0], "/")
		}

		result = append(result, info)
	}

	return result, nil
}

func (dm *DockerManager) ContainerExists(ctx context.Context, appName, color string) (bool, error) {
	containers, err := dm.findContainers(ctx, appName, color)
	if err != nil {
		return false, err
	}
	return len(containers) > 0, nil
}

func (dm *DockerManager) findContainers(ctx context.Context, appName, color string) ([]types.Container, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("dockswap.app=%s", appName))
	filterArgs.Add("label", fmt.Sprintf("dockswap.color=%s", color))
	filterArgs.Add("label", "dockswap.managed=true")

	return dm.client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
}

func buildEnvironmentVars(envMap map[string]string) []string {
	var env []string
	for key, value := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}

func applyResourceLimits(hostConfig *container.HostConfig, appConfig *config.AppConfig) error {
	// Apply memory limit
	if appConfig.Docker.MemoryLimit != "" {
		memoryBytes, err := parseMemoryLimit(appConfig.Docker.MemoryLimit)
		if err != nil {
			return fmt.Errorf("invalid memory limit %s: %w", appConfig.Docker.MemoryLimit, err)
		}
		hostConfig.Memory = memoryBytes
	}

	// Apply CPU limit
	if appConfig.Docker.CPULimit != "" {
		cpuQuota, err := parseCPULimit(appConfig.Docker.CPULimit)
		if err != nil {
			return fmt.Errorf("invalid CPU limit %s: %w", appConfig.Docker.CPULimit, err)
		}
		hostConfig.CPUQuota = cpuQuota
		hostConfig.CPUPeriod = 100000 // 100ms period
	}

	return nil
}

func applyPortMappings(hostConfig *container.HostConfig, containerConfig *container.Config, appConfig *config.AppConfig, color string) error {
	// Determine the host port based on color
	var hostPort int
	if color == "blue" {
		hostPort = appConfig.Ports.Blue
	} else {
		hostPort = appConfig.Ports.Green
	}

	// Configure port mapping
	containerConfig.ExposedPorts = nat.PortSet{
		nat.Port(fmt.Sprintf("%d/tcp", appConfig.Docker.ExposePort)): struct{}{},
	}

	hostConfig.PortBindings = nat.PortMap{
		nat.Port(fmt.Sprintf("%d/tcp", appConfig.Docker.ExposePort)): []nat.PortBinding{
			{
				HostPort: fmt.Sprintf("%d", hostPort),
			},
		},
	}

	return nil
}

func applyVolumeMounts(hostConfig *container.HostConfig, appConfig *config.AppConfig) error {
	var binds []string
	for _, volume := range appConfig.Docker.Volumes {
		binds = append(binds, volume)
	}
	hostConfig.Binds = binds
	return nil
}

func parseMemoryLimit(limit string) (int64, error) {
	// Simple memory parsing - in production you'd want more robust parsing
	// This handles formats like "512m", "1g", etc.
	if strings.HasSuffix(limit, "m") || strings.HasSuffix(limit, "M") {
		// Parse megabytes
		return 512 * 1024 * 1024, nil // Simplified
	}
	if strings.HasSuffix(limit, "g") || strings.HasSuffix(limit, "G") {
		// Parse gigabytes
		return 1024 * 1024 * 1024, nil // Simplified
	}
	return 0, fmt.Errorf("unsupported memory format: %s", limit)
}

func parseCPULimit(limit string) (int64, error) {
	// Simple CPU parsing - in production you'd want more robust parsing
	// This handles formats like "0.5", "1.0", etc.
	if limit == "0.5" {
		return 50000, nil // 50% of CPU
	}
	if limit == "1.0" {
		return 100000, nil // 100% of CPU
	}
	return 0, fmt.Errorf("unsupported CPU format: %s", limit)
}
func parseCreatedTime(created string) time.Time {
	// Docker uses RFC3339Nano format
	if t, err := time.Parse(time.RFC3339Nano, created); err == nil {
		return t
	}
	// Fallback to RFC3339
	if t, err := time.Parse(time.RFC3339, created); err == nil {
		return t
	}
	// If parsing fails, return zero time
	return time.Time{}
}

// GenerateDockerCommand generates the equivalent docker run command for a container
func (dm *DockerManager) GenerateDockerCommand(ctx context.Context, appName, color string, appConfig *config.AppConfig) (string, error) {
	containerName := fmt.Sprintf("%s-%s", appName, color)

	// Get container info to find the actual image
	containerInfo, err := dm.GetContainerInfo(ctx, appName, color)
	if err != nil {
		return "", fmt.Errorf("failed to get container info: %w", err)
	}

	var parts []string
	parts = append(parts, "docker run -d")

	// Container name
	parts = append(parts, fmt.Sprintf("--name %s", containerName))

	// Labels
	parts = append(parts, fmt.Sprintf("--label dockswap.app=%s", appName))
	parts = append(parts, fmt.Sprintf("--label dockswap.color=%s", color))
	parts = append(parts, "--label dockswap.managed=true")

	// Restart policy
	if appConfig.Docker.RestartPolicy != "" {
		parts = append(parts, fmt.Sprintf("--restart %s", appConfig.Docker.RestartPolicy))
	}

	// Resource limits
	if appConfig.Docker.MemoryLimit != "" {
		parts = append(parts, fmt.Sprintf("--memory %s", appConfig.Docker.MemoryLimit))
	}

	if appConfig.Docker.CPULimit != "" {
		parts = append(parts, fmt.Sprintf("--cpus %s", appConfig.Docker.CPULimit))
	}

	// Port mappings
	var hostPort int
	if color == "blue" {
		hostPort = appConfig.Ports.Blue
	} else {
		hostPort = appConfig.Ports.Green
	}
	parts = append(parts, fmt.Sprintf("-p %d:%d", hostPort, appConfig.Docker.ExposePort))

	// Environment variables
	for key, value := range appConfig.Docker.Environment {
		parts = append(parts, fmt.Sprintf("-e %s=%s", key, value))
	}

	// Volume mounts
	for _, volume := range appConfig.Docker.Volumes {
		parts = append(parts, fmt.Sprintf("-v %s", volume))
	}

	// Network
	if appConfig.Docker.Network != "" {
		parts = append(parts, fmt.Sprintf("--network %s", appConfig.Docker.Network))
	}

	// Image (use the actual image from running container)
	parts = append(parts, containerInfo.Image)

	return strings.Join(parts, " \\\n  "), nil
}
