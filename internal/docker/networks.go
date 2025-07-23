package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
)

type NetworkInfo struct {
	ID     string
	Name   string
	Driver string
	Scope  string
}

func (dm *DockerManager) CreateNetwork(ctx context.Context, networkName string) (*NetworkInfo, error) {
	// Check if network already exists
	exists, existingNetwork, err := dm.NetworkExists(ctx, networkName)
	if err != nil {
		return nil, fmt.Errorf("failed to check network existence: %w", err)
	}

	if exists {
		return existingNetwork, nil
	}

	// Create network
	createOptions := network.CreateOptions{
		Driver: "bridge",
		Labels: map[string]string{
			"dockswap.managed": "true",
		},
		IPAM: &network.IPAM{
			Driver: "default",
		},
	}

	resp, err := dm.client.NetworkCreate(ctx, networkName, createOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create network %s: %w", networkName, err)
	}

	return &NetworkInfo{
		ID:     resp.ID,
		Name:   networkName,
		Driver: "bridge",
		Scope:  "local",
	}, nil
}

func (dm *DockerManager) NetworkExists(ctx context.Context, networkName string) (bool, *NetworkInfo, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("name", networkName)

	networks, err := dm.client.NetworkList(ctx, network.ListOptions{
		Filters: filterArgs,
	})
	if err != nil {
		return false, nil, fmt.Errorf("failed to list networks: %w", err)
	}

	for _, net := range networks {
		if net.Name == networkName {
			return true, &NetworkInfo{
				ID:     net.ID,
				Name:   net.Name,
				Driver: net.Driver,
				Scope:  net.Scope,
			}, nil
		}
	}

	return false, nil, nil
}

func (dm *DockerManager) ConnectContainerToNetwork(ctx context.Context, networkName, containerID string) error {
	// Check if network exists
	exists, networkInfo, err := dm.NetworkExists(ctx, networkName)
	if err != nil {
		return fmt.Errorf("failed to check network existence: %w", err)
	}

	if !exists {
		return fmt.Errorf("network %s does not exist", networkName)
	}

	// Connect container to network
	err = dm.client.NetworkConnect(ctx, networkInfo.ID, containerID, &network.EndpointSettings{})
	if err != nil {
		return fmt.Errorf("failed to connect container %s to network %s: %w", containerID, networkName, err)
	}

	return nil
}

func (dm *DockerManager) EnsureNetwork(ctx context.Context, networkName string) (*NetworkInfo, error) {
	if networkName == "" {
		return nil, nil // No network configuration
	}

	// Try to create network (will return existing if it already exists)
	return dm.CreateNetwork(ctx, networkName)
}

func (dm *DockerManager) ListNetworks(ctx context.Context) ([]*NetworkInfo, error) {
	networks, err := dm.client.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	var result []*NetworkInfo
	for _, net := range networks {
		result = append(result, &NetworkInfo{
			ID:     net.ID,
			Name:   net.Name,
			Driver: net.Driver,
			Scope:  net.Scope,
		})
	}

	return result, nil
}

func (dm *DockerManager) ListDockswapNetworks(ctx context.Context) ([]*NetworkInfo, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "dockswap.managed=true")

	networks, err := dm.client.NetworkList(ctx, network.ListOptions{
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list dockswap networks: %w", err)
	}

	var result []*NetworkInfo
	for _, net := range networks {
		result = append(result, &NetworkInfo{
			ID:     net.ID,
			Name:   net.Name,
			Driver: net.Driver,
			Scope:  net.Scope,
		})
	}

	return result, nil
}
