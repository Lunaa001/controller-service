package resilience

import "sync"

type Circuit struct {
	Service      string `json:"service"`
	State        string `json:"state"`
	FailureCount int    `json:"failureCount"`
}

type Registry struct {
	mu       sync.RWMutex
	order    []string
	circuits map[string]*Circuit
}

func NewRegistry() *Registry {
	order := []string{"redis", "user-service", "extract-service", "summary-service", "persistence-service"}
	circuits := make(map[string]*Circuit, len(order))
	for _, service := range order {
		circuits[service] = &Circuit{Service: service, State: "CLOSED"}
	}
	return &Registry{order: order, circuits: circuits}
}

func (r *Registry) Snapshot() []Circuit {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Circuit, 0, len(r.order))
	for _, service := range r.order {
		c := r.circuits[service]
		out = append(out, *c)
	}
	return out
}

func (r *Registry) OpenServices() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]string, 0)
	for _, service := range r.order {
		if r.circuits[service].State == "OPEN" {
			out = append(out, service)
		}
	}
	return out
}

func (r *Registry) Services() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]string(nil), r.order...)
}

