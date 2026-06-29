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
	"strings"
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

// LoadConsulConfig loads Consul settings from environment variables.
// Supports both CONSUL_URL (e.g. "http://consul:8500") and CONSUL_HOST+CONSUL_PORT.
func LoadConsulConfig() ConsulConfig {
	// Try CONSUL_URL first (format: http://host:port)
	if consulURL := os.Getenv("CONSUL_URL"); consulURL != "" {
		// Strip protocol
		stripped := consulURL
		for _, prefix := range []string{"http://", "https://"} {
			if len(stripped) > len(prefix) && stripped[:len(prefix)] == prefix {
				stripped = stripped[len(prefix):]
				break
			}
		}
		host := stripped
		port := 8500
		// Split host:port
		for i := len(stripped) - 1; i >= 0; i-- {
			if stripped[i] == ':' {
				host = stripped[:i]
				if p, err := strconv.Atoi(stripped[i+1:]); err == nil {
					port = p
				}
				break
			}
		}
		return ConsulConfig{
			Host:  host,
			Port:  port,
			Token: env("CONSUL_TOKEN", ""),
		}
	}

	// Fallback to individual env vars
	port, _ := strconv.Atoi(env("CONSUL_PORT", "8500"))
	return ConsulConfig{
		Host:  env("CONSUL_HOST", "consul"),
		Port:  port,
		Token: env("CONSUL_TOKEN", ""),
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

// FetchTraefikTags builds Traefik-compatible tags from Consul KV sub-keys.
//
// Every leaf key under config/{serviceName}/traefik/** maps 1:1 to a Traefik
// tag by replacing "/" with "." and prefixing with "traefik.". E.g.:
//
//	config/{serviceName}/traefik/enable                                   = true
//	  -> traefik.enable=true
//	config/{serviceName}/traefik/http/routers/{name}/rule                 = Host(`...`)
//	  -> traefik.http.routers.{name}.rule=Host(`...`)
//	config/{serviceName}/traefik/http/routers/{name}/middlewares          = rate-limit@file
//	  -> traefik.http.routers.{name}.middlewares=rate-limit@file
//	config/{serviceName}/traefik/http/services/{name}/loadbalancer/server/port = 5000
//	  -> traefik.http.services.{name}.loadbalancer.server.port=5000
//
// This supports any Traefik tag without code changes — the KV tree under
// config/{service}/traefik/ is the single source of truth, seeded by the
// Consul KV seeder.
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
	var tags []string

	for _, entry := range entries {
		if entry.Value == "" {
			continue // folder entry, no value
		}
		relativeKey := entry.Key
		if len(entry.Key) > len(prefix) {
			relativeKey = entry.Key[len(prefix):]
		} else {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(entry.Value)
		if err != nil {
			continue
		}
		tagName := "traefik." + strings.ReplaceAll(relativeKey, "/", ".")
		tags = append(tags, fmt.Sprintf("%s=%s", tagName, string(decoded)))
	}

	if len(tags) > 0 {
		log.Printf("✓ Loaded %d Traefik tags from Consul KV for %s", len(tags), serviceName)
	}
	return tags
}

// RegisterService registers this service instance with Consul.
// If tags is nil, they are fetched automatically from Consul KV.
//
// Retries with a fixed backoff because the Consul stack and this service's
// stack are deployed as independent `docker compose` projects — there is no
// cross-project `depends_on`, so Consul may not be ready yet on first try.
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
	client := &http.Client{Timeout: 5 * time.Second}

	const retries = 5
	const delay = 2 * time.Second
	var lastErr error
	for attempt := 1; attempt <= retries; attempt++ {
		req, err := http.NewRequest("PUT", reqURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to create consul request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Consul-Token", c.Token)

		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				log.Printf("✓ Registered in Consul: %s (id=%s, addr=%s:%d, tags=%d)", serviceName, serviceID, containerIP, servicePort, len(tags))
				return nil
			}
			lastErr = fmt.Errorf("consul registration returned status %d", resp.StatusCode)
		} else {
			lastErr = fmt.Errorf("consul registration failed: %w", err)
		}

		if attempt < retries {
			log.Printf("warning: consul registration attempt %d/%d failed for %s: %v, retrying in %s", attempt, retries, serviceName, lastErr, delay)
			time.Sleep(delay)
		}
	}
	return lastErr
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
