package discovery

import (
	"context"
	"fmt"
	"strings"
)

// DockerDiscoverer finds running containers as targets.
// It uses the Docker socket or environment variables.
type DockerDiscoverer struct {
	host string // Docker daemon host (e.g., unix:///var/run/docker.sock)
}

// NewDocker creates a Docker discoverer.
func NewDocker(host string) *DockerDiscoverer {
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}
	return &DockerDiscoverer{host: host}
}

// Name returns the discoverer name.
func (d *DockerDiscoverer) Name() string {
	return "docker"
}

// Discover returns containers with exposed ports as targets.
// In a real implementation, this would use the Docker SDK.
// For now, it returns an empty list as a placeholder.
func (d *DockerDiscoverer) Discover(ctx context.Context) ([]Target, error) {
	// TODO: Implement Docker SDK integration.
	// Example pseudo-code:
	// cli, err := client.NewClientWithOpts(client.WithHost(d.host))
	// containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	// for _, c := range containers {
	//     for _, p := range c.Ports {
	//         targets = append(targets, Target{
	//             Name:    strings.TrimPrefix(c.Names[0], "/"),
	//             Address: fmt.Sprintf("%s:%d", p.IP, p.PublicPort),
	//             Labels:  c.Labels,
	//         })
	//     }
	// }
	return nil, nil
}

// containerName extracts the first name without leading slash.
func containerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}

// formatAddress formats IP:port address.
func formatAddress(ip string, port int) string {
	if ip == "0.0.0.0" || ip == "" {
		ip = "localhost"
	}
	return fmt.Sprintf("%s:%d", ip, port)
}
