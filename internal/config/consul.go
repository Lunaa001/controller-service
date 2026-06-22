package config

import (
	"bytes"
	"encoding/base64"
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

// consulKVEntry represents a single entry from Consul KV API
type consulKVEntry struct {
	Key   string `json:"Key"`
	Value string `json:"Value"` // base64 encoded
}

// LoadConsulConfig loads Consul settings from environment variables
func LoadConsulConfig() ConsulConfig {
	port, _ := strconv.Atoi(env("CONSUL_PORT", "8500"))
	return ConsulConfig{
		Host:  env("CONSUL_HOST", "consul"),
		Port:  port,
		Token: env("CONSUL_TOKEN", "2be22662-4819-4a0c-81d9-b3f50c4c389c"),
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

// FetchKVConfig reads all config keys for a service from Consul KV
func (c ConsulConfig) FetchKVConfig(serviceName string) map[string]string {
	url := fmt.Sprintf("http://%s:%d/v1/kv/config/%s/?recurse", c.Host, c.Port, serviceName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("warning: failed to create consul KV request: %v", err)
		return nil
	}
	req.Header.Set("X-Consul-Token", c.Token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("warning: failed to read Consul KV for %s: %v", serviceName, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("warning: Consul KV returned status %d for %s", resp.StatusCode, serviceName)
		return nil
	}

	var entries []consulKVEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		log.Printf("warning: failed to decode Consul KV response: %v", err)
		return nil
	}

	prefix := fmt.Sprintf("config/%s/", serviceName)
	config := make(map[string]string)

	for _, entry := range entries {
		if entry.Value == "" {
			continue
		}
		relativeKey := entry.Key
		if len(entry.Key) > len(prefix) {
			relativeKey = entry.Key[len(prefix):]
		}
		// Skip traefik sub-keys (handled by FetchTraefikTags)
		if len(relativeKey) > 8 && relativeKey[:8] == "traefik/" {
			continue
		}

		decoded, err := base64.StdEncoding.DecodeString(entry.Value)
		if err != nil {
			continue
		}
		config[relativeKey] = string(decoded)
	}

	log.Printf("✓ Loaded %d config keys from Consul KV for %s", len(config), serviceName)
	return config
}

// FetchTraefikTags builds Traefik-compatible tags from Consul KV sub-keys
func (c ConsulConfig) FetchTraefikTags(serviceName string) []string {
	url := fmt.Sprintf("http://%s:%d/v1/kv/config/%s/traefik/?recurse", c.Host, c.Port, serviceName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("X-Consul-Token", c.Token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("warning: failed to read Traefik tags from Consul KV for %s: %v", serviceName, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil
	}

	var entries []consulKVEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil
	}

	prefix := fmt.Sprintf("config/%s/traefik/", serviceName)
	traefikKV := make(map[string]string)

	for _, entry := range entries {
		if entry.Value == "" {
			continue
		}
		relativeKey := entry.Key
		if len(entry.Key) > len(prefix) {
			relativeKey = entry.Key[len(prefix):]
		}
		decoded, err := base64.StdEncoding.DecodeString(entry.Value)
		if err != nil {
			continue
		}
		traefikKV[relativeKey] = string(decoded)
	}

	if len(traefikKV) == 0 {
		return nil
	}

	var tags []string
	if v, ok := traefikKV["enable"]; ok {
		tags = append(tags, fmt.Sprintf("traefik.enable=%s", v))
	}
	if v, ok := traefikKV["router_rule"]; ok {
		tags = append(tags, fmt.Sprintf("traefik.http.routers.%s.rule=%s", serviceName, v))
	}
	if v, ok := traefikKV["entrypoints"]; ok {
		tags = append(tags, fmt.Sprintf("traefik.http.routers.%s.entryPoints=%s", serviceName, v))
	}
	if v, ok := traefikKV["lb_port"]; ok {
		tags = append(tags, fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=%s", serviceName, v))
	}

	log.Printf("✓ Loaded %d Traefik tags from Consul KV for %s", len(tags), serviceName)
	return tags
}

// RegisterService registers this service instance with Consul.
// If tags is nil, they are fetched automatically from Consul KV.
func (c ConsulConfig) RegisterService(serviceName string, servicePort int, healthCheckPath string, tags []string) error {
	// If no tags provided, fetch from Consul KV
	if tags == nil {
		tags = c.FetchTraefikTags(serviceName)
		if tags == nil {
			tags = []string{}
		}
	}

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

	reqURL := fmt.Sprintf("http://%s:%d/v1/agent/service/register", c.Host, c.Port)
	req, err := http.NewRequest("PUT", reqURL, bytes.NewReader(body))
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

	log.Printf("✓ Registered in Consul: %s (id=%s, addr=%s:%d, tags=%d)", serviceName, serviceID, containerIP, servicePort, len(tags))
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
