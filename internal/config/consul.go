package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

// ConsulConfig holds Consul connection settings
type ConsulConfig struct {
	Host  string
	Port  int
	Token string
}

// consulServiceRegistration is the JSON payload for Consul's /v1/agent/service/register
type consulServiceRegistration struct {
	ID      string            `json:"ID"`
	Name    string            `json:"Name"`
	Address string            `json:"Address"`
	Port    int               `json:"Port"`
	Tags    []string          `json:"Tags"`
	Check   consulHealthCheck `json:"Check"`
}

type consulHealthCheck struct {
	HTTP                           string `json:"HTTP"`
	Interval                       string `json:"Interval"`
	Timeout                        string `json:"Timeout"`
	DeregisterCriticalServiceAfter string `json:"DeregisterCriticalServiceAfter"`
}

// LoadConsulConfig loads Consul settings from environment variables
func LoadConsulConfig() ConsulConfig {
	port, _ := strconv.Atoi(env("CONSUL_PORT", "8500"))
	return ConsulConfig{
		Host:  env("CONSUL_HOST", "consul"),
		Port:  port,
		Token: env("CONSUL_TOKEN", "ac80cdb0-2ca6-4182-ae14-15158c33c095"),
	}
}

// getContainerIP returns the container's IP address for Consul registration
func getContainerIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}
	// Fallback: resolve hostname
	hostname, err := os.Hostname()
	if err != nil {
		return "127.0.0.1"
	}
	ips, err := net.LookupHost(hostname)
	if err != nil || len(ips) == 0 {
		return "127.0.0.1"
	}
	return ips[0]
}

// RegisterService registers this service instance with Consul
func (c ConsulConfig) RegisterService(serviceName string, servicePort int, healthCheckPath string, tags []string) error {
	containerIP := getContainerIP()
	serviceID := fmt.Sprintf("%s-%s-%d", serviceName, containerIP, servicePort)

	registration := consulServiceRegistration{
		ID:      serviceID,
		Name:    serviceName,
		Address: containerIP,
		Port:    servicePort,
		Tags:    tags,
		Check: consulHealthCheck{
			HTTP:                           fmt.Sprintf("http://%s:%d%s", containerIP, servicePort, healthCheckPath),
			Interval:                       "15s",
			Timeout:                        "5s",
			DeregisterCriticalServiceAfter: "90s",
		},
	}

	body, err := json.Marshal(registration)
	if err != nil {
		return fmt.Errorf("failed to marshal consul registration: %w", err)
	}

	url := fmt.Sprintf("http://%s:%d/v1/agent/service/register", c.Host, c.Port)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create consul request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Consul-Token", c.Token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("consul registration failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("consul registration returned status %d", resp.StatusCode)
	}

	log.Printf("✓ Registered in Consul: %s (id=%s, addr=%s:%d)", serviceName, serviceID, containerIP, servicePort)
	return nil
}

// DeregisterService deregisters this service instance from Consul
func (c ConsulConfig) DeregisterService(serviceName string, servicePort int) error {
	containerIP := getContainerIP()
	serviceID := fmt.Sprintf("%s-%s-%d", serviceName, containerIP, servicePort)

	url := fmt.Sprintf("http://%s:%d/v1/agent/service/deregister/%s", c.Host, c.Port, serviceID)
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create consul deregister request: %w", err)
	}
	req.Header.Set("X-Consul-Token", c.Token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("consul deregistration failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("consul deregistration returned status %d", resp.StatusCode)
	}

	log.Printf("✓ Deregistered from Consul: %s (id=%s)", serviceName, serviceID)
	return nil
}
