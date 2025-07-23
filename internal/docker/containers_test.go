package docker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"dockswap/internal/config"
)

func createTestAppConfig() *config.AppConfig {
	return &config.AppConfig{
		Name: "test-app",
		Docker: config.Docker{
			RestartPolicy: "unless-stopped",
			MemoryLimit:   "512m",
			CPULimit:      "0.5",
			Environment: map[string]string{
				"ENV": "test",
			},
			Volumes:    []string{"/host:/container"},
			ExposePort: 8080,
			Network:    "test-network",
		},
		Ports: config.Ports{
			Blue:  8081,
			Green: 8082,
		},
		HealthCheck: config.HealthCheck{
			Endpoint:         "/health",
			Method:           "GET",
			Timeout:          5 * time.Second,
			Interval:         2 * time.Second,
			Retries:          3,
			SuccessThreshold: 2,
			ExpectedStatus:   200,
		},
		Deployment: config.Deployment{
			StopTimeout: 15 * time.Second,
		},
	}
}

func TestDockerManager_CreateContainer(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		dm := NewDockerManager(mockClient)
		appConfig := createTestAppConfig()

		expectedResp := container.CreateResponse{
			ID: "container123",
		}

		mockClient.On("ContainerCreate",
			mock.Anything,
			mock.MatchedBy(func(config *container.Config) bool {
				return config.Image == "nginx:1.21" &&
					len(config.Env) > 0 &&
					config.Labels["dockswap.app"] == "test-app" &&
					config.Labels["dockswap.color"] == "blue"
			}),
			mock.MatchedBy(func(hostConfig *container.HostConfig) bool {
				return hostConfig.RestartPolicy.Name == "unless-stopped" &&
					len(hostConfig.Binds) > 0
			}),
			mock.Anything,
			"test-app-blue",
		).Return(expectedResp, nil)

		containerInfo, err := dm.CreateContainer(context.Background(), "test-app", "blue", "nginx:1.21", appConfig)

		assert.NoError(t, err)
		assert.NotNil(t, containerInfo)
		assert.Equal(t, "container123", containerInfo.ID)
		assert.Equal(t, "test-app-blue", containerInfo.Name)
		assert.Equal(t, "nginx:1.21", containerInfo.Image)
		mockClient.AssertExpectations(t)
	})

	t.Run("creation failure", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		dm := NewDockerManager(mockClient)
		appConfig := createTestAppConfig()

		mockClient.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(container.CreateResponse{}, errors.New("creation failed"))

		containerInfo, err := dm.CreateContainer(context.Background(), "test-app", "blue", "nginx:1.21", appConfig)

		assert.Error(t, err)
		assert.Nil(t, containerInfo)
		assert.Contains(t, err.Error(), "failed to create container")
		mockClient.AssertExpectations(t)
	})
}

func TestDockerManager_StartContainer(t *testing.T) {
	t.Run("successful start", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		dm := NewDockerManager(mockClient)

		mockClient.On("ContainerStart", mock.Anything, "container123", mock.Anything).Return(nil)

		err := dm.StartContainer(context.Background(), "container123")

		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("start failure", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		dm := NewDockerManager(mockClient)

		mockClient.On("ContainerStart", mock.Anything, "container123", mock.Anything).
			Return(errors.New("start failed"))

		err := dm.StartContainer(context.Background(), "container123")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to start container")
		mockClient.AssertExpectations(t)
	})
}

func TestDockerManager_StopContainer(t *testing.T) {
	t.Run("successful stop", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		dm := NewDockerManager(mockClient)

		containers := []types.Container{
			{
				ID:    "container123",
				Names: []string{"/test-app-blue"},
			},
		}

		mockClient.On("ContainerList", mock.Anything, mock.Anything).Return(containers, nil)
		mockClient.On("ContainerStop", mock.Anything, "container123", mock.Anything).Return(nil)

		err := dm.StopContainer(context.Background(), "test-app", "blue", 15*time.Second)

		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("container not found", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		dm := NewDockerManager(mockClient)

		mockClient.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{}, nil)

		err := dm.StopContainer(context.Background(), "test-app", "blue", 15*time.Second)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "container test-app-blue not found")
		mockClient.AssertExpectations(t)
	})
}

func TestDockerManager_GetContainerInfo(t *testing.T) {
	t.Run("successful inspect", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		dm := NewDockerManager(mockClient)

		containers := []types.Container{
			{
				ID:    "container123",
				Names: []string{"/test-app-blue"},
			},
		}

		containerJSON := types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				ID:   "container123",
				Name: "/test-app-blue",
				State: &types.ContainerState{
					Status: "running",
					Health: &types.Health{
						Status: "healthy",
					},
				},
				Created: time.Now().Format(time.RFC3339Nano),
			},
			Config: &container.Config{
				Image: "nginx:1.21",
			},
			NetworkSettings: &types.NetworkSettings{},
		}

		mockClient.On("ContainerList", mock.Anything, mock.Anything).Return(containers, nil)
		mockClient.On("ContainerInspect", mock.Anything, "container123").Return(containerJSON, nil)

		info, err := dm.GetContainerInfo(context.Background(), "test-app", "blue")

		assert.NoError(t, err)
		assert.NotNil(t, info)
		assert.Equal(t, "container123", info.ID)
		assert.Equal(t, "test-app-blue", info.Name)
		assert.Equal(t, "nginx:1.21", info.Image)
		assert.Equal(t, "running", info.Status)
		assert.Equal(t, "healthy", info.Health)
		mockClient.AssertExpectations(t)
	})

	t.Run("container not found", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		dm := NewDockerManager(mockClient)

		mockClient.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{}, nil)

		info, err := dm.GetContainerInfo(context.Background(), "test-app", "blue")

		assert.Error(t, err)
		assert.Nil(t, info)
		assert.Contains(t, err.Error(), "container test-app-blue not found")
		mockClient.AssertExpectations(t)
	})
}

func TestDockerManager_ContainerExists(t *testing.T) {
	t.Run("container exists", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		dm := NewDockerManager(mockClient)

		containers := []types.Container{
			{
				ID:    "container123",
				Names: []string{"/test-app-blue"},
			},
		}

		mockClient.On("ContainerList", mock.Anything, mock.Anything).Return(containers, nil)

		exists, err := dm.ContainerExists(context.Background(), "test-app", "blue")

		assert.NoError(t, err)
		assert.True(t, exists)
		mockClient.AssertExpectations(t)
	})

	t.Run("container does not exist", func(t *testing.T) {
		mockClient := new(MockDockerClient)
		dm := NewDockerManager(mockClient)

		mockClient.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{}, nil)

		exists, err := dm.ContainerExists(context.Background(), "test-app", "blue")

		assert.NoError(t, err)
		assert.False(t, exists)
		mockClient.AssertExpectations(t)
	})
}

func TestParseMemoryLimit(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"512m", 512 * 1024 * 1024, false},
		{"1g", 1024 * 1024 * 1024, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseMemoryLimit(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseCPULimit(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"0.5", 50000, false},
		{"1.0", 100000, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseCPULimit(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
