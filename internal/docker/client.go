package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
)

// DockerClient interface for testability
type DockerClient interface {
	// Container operations
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)

	// Network operations
	NetworkCreate(ctx context.Context, name string, options network.CreateOptions) (network.CreateResponse, error)
	NetworkList(ctx context.Context, options network.ListOptions) ([]network.Inspect, error)
	NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error

	// System operations
	Ping(ctx context.Context) (types.Ping, error)
	Info(ctx context.Context) (system.Info, error)

	// Cleanup
	Close() error
}

// RealDockerClient wraps the official Docker client
type RealDockerClient struct {
	client *client.Client
}

func NewDockerClient() (*RealDockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &RealDockerClient{client: cli}, nil
}

func (r *RealDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (container.CreateResponse, error) {
	return r.client.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, containerName)
}

func (r *RealDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	return r.client.ContainerStart(ctx, containerID, options)
}

func (r *RealDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	return r.client.ContainerStop(ctx, containerID, options)
}

func (r *RealDockerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	return r.client.ContainerRemove(ctx, containerID, options)
}

func (r *RealDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	return r.client.ContainerList(ctx, options)
}

func (r *RealDockerClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	return r.client.ContainerInspect(ctx, containerID)
}

func (r *RealDockerClient) NetworkCreate(ctx context.Context, name string, options network.CreateOptions) (network.CreateResponse, error) {
	return r.client.NetworkCreate(ctx, name, options)
}

func (r *RealDockerClient) NetworkList(ctx context.Context, options network.ListOptions) ([]network.Inspect, error) {
	return r.client.NetworkList(ctx, options)
}

func (r *RealDockerClient) NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error {
	return r.client.NetworkConnect(ctx, networkID, containerID, config)
}

func (r *RealDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	return r.client.Ping(ctx)
}

func (r *RealDockerClient) Info(ctx context.Context) (system.Info, error) {
	return r.client.Info(ctx)
}

func (r *RealDockerClient) Close() error {
	return r.client.Close()
}

// DockerManager provides high-level Docker operations
type DockerManager struct {
	client DockerClient
}

func NewDockerManager(client DockerClient) *DockerManager {
	return &DockerManager{
		client: client,
	}
}

func (dm *DockerManager) ValidateConnection(ctx context.Context) error {
	_, err := dm.client.Ping(ctx)
	if err != nil {
		return fmt.Errorf("docker daemon not accessible: %w", err)
	}
	return nil
}

func (dm *DockerManager) Close() error {
	return dm.client.Close()
}
