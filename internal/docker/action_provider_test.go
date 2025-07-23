package docker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"dockswap/internal/config"
)

// MockCaddyManager for testing
type MockCaddyManager struct {
	mock.Mock
}

func (m *MockCaddyManager) ReloadCaddy() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockCaddyManager) GenerateConfig(configs map[string]*config.AppConfig, states interface{}) error {
	args := m.Called(configs, states)
	return args.Error(0)
}

func TestDockerActionProvider_StartContainer(t *testing.T) {
	t.Run("successful container start", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		mockCaddy := new(MockCaddyManager)
		dm := NewDockerManager(mockClient)

		configs := map[string]*config.AppConfig{
			"test-app": createTestAppConfig(),
		}

		actionProvider := NewDockerActionProvider(dm, mockCaddy, configs)

		// Mock network creation
		mockClient.On("NetworkList", mock.Anything, mock.Anything).Return([]network.Inspect{{Name: "test-network", ID: "net123", Driver: "bridge", Scope: "local"}}, nil)

		// Mock container operations
		mockClient.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{}, nil)
		mockClient.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "test-app-blue").
			Return(container.CreateResponse{ID: "container123"}, nil)
		mockClient.On("ContainerStart", mock.Anything, "container123", mock.Anything).Return(nil)
		mockClient.On("NetworkConnect", mock.Anything, "net123", "container123", mock.Anything).Return(nil)

		err := actionProvider.StartContainer("test-app", "blue", "nginx:1.21")

		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("app config not found", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		mockCaddy := new(MockCaddyManager)
		dm := NewDockerManager(mockClient)

		configs := map[string]*config.AppConfig{}
		actionProvider := NewDockerActionProvider(dm, mockCaddy, configs)

		err := actionProvider.StartContainer("nonexistent-app", "blue", "nginx:1.21")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no configuration found for app nonexistent-app")
	})

	t.Run("container creation failure", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		mockCaddy := new(MockCaddyManager)
		dm := NewDockerManager(mockClient)

		configs := map[string]*config.AppConfig{
			"test-app": createTestAppConfig(),
		}
		actionProvider := NewDockerActionProvider(dm, mockCaddy, configs)

		// Mock network creation
		mockClient.On("NetworkList", mock.Anything, mock.Anything).Return([]network.Inspect{}, nil)
		mockClient.On("NetworkCreate", mock.Anything, "test-network", mock.Anything).
			Return(network.CreateResponse{ID: "net123"}, nil)

		// Mock container operations
		mockClient.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{}, nil)
		mockClient.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "test-app-blue").
			Return(container.CreateResponse{}, errors.New("creation failed"))

		err := actionProvider.StartContainer("test-app", "blue", "nginx:1.21")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create container")
		mockClient.AssertExpectations(t)
	})
}

func TestDockerActionProvider_CheckHealth(t *testing.T) {
	t.Run("healthy container", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		mockCaddy := new(MockCaddyManager)
		dm := NewDockerManager(mockClient)

		configs := map[string]*config.AppConfig{
			"test-app": createTestAppConfig(),
		}
		configs["test-app"].HealthCheck.Endpoint = ""
		actionProvider := NewDockerActionProvider(dm, mockCaddy, configs)

		// Mock container inspection
		containers := []types.Container{{ID: "container123"}}
		containerJSON := types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				ID: "container123",
				State: &types.ContainerState{
					Status: "running",
					Health: &types.Health{Status: "healthy"},
				},
			},
			Config:          &container.Config{Image: "nginx:1.21"},
			NetworkSettings: &types.NetworkSettings{},
		}

		mockClient.On("ContainerList", mock.Anything, mock.Anything).Return(containers, nil)
		mockClient.On("ContainerInspect", mock.Anything, "container123").Return(containerJSON, nil)

		healthy, err := actionProvider.CheckHealth("test-app", "blue")

		assert.NoError(t, err)
		assert.True(t, healthy)
		mockClient.AssertExpectations(t)
	})

	t.Run("unhealthy container", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		mockCaddy := new(MockCaddyManager)
		dm := NewDockerManager(mockClient)

		configs := map[string]*config.AppConfig{
			"test-app": createTestAppConfig(),
		}
		actionProvider := NewDockerActionProvider(dm, mockCaddy, configs)

		// Mock container inspection
		containers := []types.Container{{ID: "container123"}}
		containerJSON := types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				ID: "container123",
				State: &types.ContainerState{
					Status: "exited",
				},
			},
			Config:          &container.Config{Image: "nginx:1.21"},
			NetworkSettings: &types.NetworkSettings{},
		}

		mockClient.On("ContainerList", mock.Anything, mock.Anything).Return(containers, nil)
		mockClient.On("ContainerInspect", mock.Anything, "container123").Return(containerJSON, nil)

		healthy, err := actionProvider.CheckHealth("test-app", "blue")

		assert.NoError(t, err)
		assert.False(t, healthy)
		mockClient.AssertExpectations(t)
	})
}

func TestDockerActionProvider_StopContainer(t *testing.T) {
	t.Run("successful container stop", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		mockCaddy := new(MockCaddyManager)
		dm := NewDockerManager(mockClient)

		configs := map[string]*config.AppConfig{
			"test-app": createTestAppConfig(),
		}
		actionProvider := NewDockerActionProvider(dm, mockCaddy, configs)

		// Mock container operations
		containers := []types.Container{{ID: "container123"}}
		mockClient.On("ContainerList", mock.Anything, mock.Anything).Return(containers, nil).Twice()
		mockClient.On("ContainerStop", mock.Anything, "container123", mock.Anything).Return(nil)
		mockClient.On("ContainerRemove", mock.Anything, "container123", mock.Anything).Return(nil)

		err := actionProvider.StopContainer("test-app", "blue")

		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("container not found", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		mockCaddy := new(MockCaddyManager)
		dm := NewDockerManager(mockClient)

		configs := map[string]*config.AppConfig{
			"test-app": createTestAppConfig(),
		}
		actionProvider := NewDockerActionProvider(dm, mockCaddy, configs)

		mockClient.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{}, nil)

		err := actionProvider.StopContainer("test-app", "blue")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "container test-app-blue not found")
		mockClient.AssertExpectations(t)
	})
}

func TestDockerActionProvider_UpdateCaddy(t *testing.T) {
	t.Run("successful caddy update", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		mockCaddy := new(MockCaddyManager)
		dm := NewDockerManager(mockClient)

		configs := map[string]*config.AppConfig{
			"test-app": createTestAppConfig(),
		}
		actionProvider := NewDockerActionProvider(dm, mockCaddy, configs)

		mockCaddy.On("ReloadCaddy").Return(nil)

		err := actionProvider.UpdateCaddy("test-app", "blue")

		assert.NoError(t, err)
		mockCaddy.AssertExpectations(t)
	})

	t.Run("caddy manager not available", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		dm := NewDockerManager(mockClient)

		configs := map[string]*config.AppConfig{
			"test-app": createTestAppConfig(),
		}
		actionProvider := NewDockerActionProvider(dm, nil, configs)

		err := actionProvider.UpdateCaddy("test-app", "blue")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "caddy manager not available")
	})

	t.Run("caddy reload failure", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		mockCaddy := new(MockCaddyManager)
		dm := NewDockerManager(mockClient)

		configs := map[string]*config.AppConfig{
			"test-app": createTestAppConfig(),
		}
		actionProvider := NewDockerActionProvider(dm, mockCaddy, configs)

		mockCaddy.On("ReloadCaddy").Return(errors.New("reload failed"))

		err := actionProvider.UpdateCaddy("test-app", "blue")

		assert.Error(t, err)
		mockCaddy.AssertExpectations(t)
	})
}

func TestDockerActionProvider_DrainConnections(t *testing.T) {
	t.Run("successful drain", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		mockCaddy := new(MockCaddyManager)
		dm := NewDockerManager(mockClient)

		configs := map[string]*config.AppConfig{
			"test-app": createTestAppConfig(),
		}
		actionProvider := NewDockerActionProvider(dm, mockCaddy, configs)

		// Test with very short timeout for quick test
		err := actionProvider.DrainConnections("test-app", "blue", 1*time.Millisecond)

		assert.NoError(t, err)
	})

	t.Run("context cancelled during drain", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		mockCaddy := new(MockCaddyManager)
		dm := NewDockerManager(mockClient)

		configs := map[string]*config.AppConfig{
			"test-app": createTestAppConfig(),
		}
		actionProvider := NewDockerActionProvider(dm, mockCaddy, configs)

		// Set a cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		actionProvider.SetContext(ctx)

		err := actionProvider.DrainConnections("test-app", "blue", 1*time.Second)

		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}
