package docker

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDockerClient is a mock implementation of DockerClient
type MockDockerClient struct {
	mock.Mock
}

func (m *MockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (container.CreateResponse, error) {
	args := m.Called(ctx, config, hostConfig, networkingConfig, containerName)
	return args.Get(0).(container.CreateResponse), args.Error(1)
}

func (m *MockDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *MockDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *MockDockerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *MockDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	args := m.Called(ctx, options)
	return args.Get(0).([]types.Container), args.Error(1)
}

func (m *MockDockerClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	args := m.Called(ctx, containerID)
	return args.Get(0).(types.ContainerJSON), args.Error(1)
}

func (m *MockDockerClient) NetworkCreate(ctx context.Context, name string, options network.CreateOptions) (network.CreateResponse, error) {
	args := m.Called(ctx, name, options)
	return args.Get(0).(network.CreateResponse), args.Error(1)
}

func (m *MockDockerClient) NetworkList(ctx context.Context, options network.ListOptions) ([]network.Inspect, error) {
	args := m.Called(ctx, options)
	return args.Get(0).([]network.Inspect), args.Error(1)
}

func (m *MockDockerClient) NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error {
	args := m.Called(ctx, networkID, containerID, config)
	return args.Error(0)
}

func (m *MockDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	args := m.Called(ctx)
	return args.Get(0).(types.Ping), args.Error(1)
}

func (m *MockDockerClient) Info(ctx context.Context) (system.Info, error) {
	args := m.Called(ctx)
	return args.Get(0).(system.Info), args.Error(1)
}

func (m *MockDockerClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestDockerManager_ValidateConnection(t *testing.T) {
	t.Run("successful connection", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		mockClient.On("Ping", mock.Anything).Return(types.Ping{}, nil)

		dm := NewDockerManager(mockClient)
		err := dm.ValidateConnection(context.Background())

		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("connection failure", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		mockClient.On("Ping", mock.Anything).Return(types.Ping{}, errors.New("connection failed"))

		dm := NewDockerManager(mockClient)
		err := dm.ValidateConnection(context.Background())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not accessible")
		mockClient.AssertExpectations(t)
	})
}

func TestNewDockerManager(t *testing.T) {
	mockClient := new(MockDockerClient)
	dm := NewDockerManager(mockClient)

	assert.NotNil(t, dm)
	assert.Equal(t, mockClient, dm.client)
}

func TestDockerManager_Close(t *testing.T) {
	mockClient := new(MockDockerClient)
	mockClient.On("Close").Return(nil)

	dm := NewDockerManager(mockClient)
	err := dm.Close()

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}
