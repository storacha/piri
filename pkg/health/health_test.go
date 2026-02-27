package health

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewChecker_FullMode(t *testing.T) {
	c := NewChecker(ModeFull)

	assert.Equal(t, ModeFull, c.Mode())
	assert.True(t, c.IsReady(), "Full mode should be ready by default")
}

func TestNewChecker_InitMode(t *testing.T) {
	c := NewChecker(ModeInit)

	assert.Equal(t, ModeInit, c.Mode())
	assert.False(t, c.IsReady(), "Init mode should not be ready by default")
}

func TestChecker_SetReady(t *testing.T) {
	c := NewChecker(ModeInit)
	assert.False(t, c.IsReady())

	c.SetReady(true)
	assert.True(t, c.IsReady())

	c.SetReady(false)
	assert.False(t, c.IsReady())
}

func TestChecker_LivenessCheck(t *testing.T) {
	c := NewChecker(ModeInit)

	resp := c.LivenessCheck()
	assert.Equal(t, StatusOK, resp.Status)
	assert.NotEmpty(t, resp.Version)
	assert.NotZero(t, resp.Timestamp)
}

func TestChecker_ReadinessCheck_Ready(t *testing.T) {
	c := NewChecker(ModeFull)

	resp := c.ReadinessCheck()
	assert.Equal(t, StatusOK, resp.Status)
	assert.Equal(t, "full", resp.Mode)
	assert.NotEmpty(t, resp.Version)
}

func TestChecker_ReadinessCheck_NotReady(t *testing.T) {
	c := NewChecker(ModeInit)

	resp := c.ReadinessCheck()
	assert.Equal(t, StatusFailed, resp.Status)
	assert.Equal(t, "init", resp.Mode)
	assert.NotEmpty(t, resp.Version)
}

func TestChecker_HealthCheck_Healthy(t *testing.T) {
	c := NewChecker(ModeFull)

	resp := c.HealthCheck()
	assert.Equal(t, StatusOK, resp.Status)
	assert.Equal(t, "full", resp.Mode)
	assert.Len(t, resp.Checks, 2)
	assert.Equal(t, "liveness", resp.Checks[0].Name)
	assert.Equal(t, StatusOK, resp.Checks[0].Status)
	assert.Equal(t, "readiness", resp.Checks[1].Name)
	assert.Equal(t, StatusOK, resp.Checks[1].Status)
}

func TestChecker_HealthCheck_NotHealthy(t *testing.T) {
	c := NewChecker(ModeInit)

	resp := c.HealthCheck()
	assert.Equal(t, StatusFailed, resp.Status)
	assert.Equal(t, "init", resp.Mode)
	assert.Len(t, resp.Checks, 2)
	assert.Equal(t, "liveness", resp.Checks[0].Name)
	assert.Equal(t, StatusOK, resp.Checks[0].Status)
	assert.Equal(t, "readiness", resp.Checks[1].Name)
	assert.Equal(t, StatusFailed, resp.Checks[1].Status)
}
