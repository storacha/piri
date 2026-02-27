package health

import (
	"sync"
	"time"

	"github.com/storacha/piri/pkg/build"
)

// ServerMode indicates the operational mode of the server
type ServerMode string

const (
	// ModeInit indicates the server is running in initialization mode
	ModeInit ServerMode = "init"
	// ModeFull indicates the server is running in full PDP+UCAN mode
	ModeFull ServerMode = "full"
)

// Status represents the health status
type Status string

const (
	StatusOK     Status = "ok"
	StatusFailed Status = "failed"
)

// Response represents a health check response
type Response struct {
	Status    Status    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Mode      string    `json:"mode,omitempty"`
	Checks    []Check   `json:"checks,omitempty"`
}

// Check represents an individual health check result
type Check struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
}

// Checker provides health check functionality
type Checker struct {
	mode  ServerMode
	mu    sync.RWMutex
	ready bool
}

// NewChecker creates a new health checker
func NewChecker(mode ServerMode) *Checker {
	return &Checker{
		mode:  mode,
		ready: mode != ModeInit, // Ready by default except in init mode
	}
}

// Mode returns the server mode
func (c *Checker) Mode() ServerMode {
	return c.mode
}

// SetReady sets the readiness state
func (c *Checker) SetReady(ready bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ready = ready
}

// IsReady returns the readiness state
func (c *Checker) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ready
}

// LivenessCheck performs a liveness check
func (c *Checker) LivenessCheck() Response {
	return Response{
		Status:    StatusOK,
		Timestamp: time.Now().UTC(),
		Version:   build.Version,
	}
}

// ReadinessCheck performs a readiness check
func (c *Checker) ReadinessCheck() Response {
	status := StatusOK
	if !c.IsReady() {
		status = StatusFailed
	}

	return Response{
		Status:    status,
		Timestamp: time.Now().UTC(),
		Version:   build.Version,
		Mode:      string(c.mode),
	}
}

// HealthCheck performs a combined health check
func (c *Checker) HealthCheck() Response {
	liveness := c.LivenessCheck()
	readiness := c.ReadinessCheck()

	status := StatusOK
	if readiness.Status != StatusOK {
		status = StatusFailed
	}

	return Response{
		Status:    status,
		Timestamp: time.Now().UTC(),
		Version:   build.Version,
		Mode:      string(c.mode),
		Checks: []Check{
			{Name: "liveness", Status: liveness.Status},
			{Name: "readiness", Status: readiness.Status},
		},
	}
}
