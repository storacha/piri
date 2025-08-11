package client

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// setTestNodeURL configures viper with the host:port of the provided server
func setTestNodeURL(t *testing.T, ts *httptest.Server) func() {
	t.Helper()
	parsed, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	prev := viper.GetString("node_url")
	viper.Set("node_url", parsed.Host)
	return func() { viper.Set("node_url", prev) }
}

func TestLogList_PrintsSubsystemsAndLevels(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/log/subsystems", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		// Return a stable order
		_, _ = w.Write([]byte(`{"subsystems":["alpha","beta"]}`))
	})
	mux.HandleFunc("/log/level", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		_, _ = w.Write([]byte(`{"levels":{"alpha":"Info","beta":"Debug"}}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	restore := setTestNodeURL(t, ts)
	defer restore()

	var out bytes.Buffer
	Cmd.SetOut(&out)
	Cmd.SetErr(&out)
	Cmd.SetArgs([]string{"log", "list"})

	if err := Cmd.Execute(); err != nil {
		t.Fatalf("log list failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "Info") {
		t.Fatalf("output missing alpha Info: %q", got)
	}
	if !strings.Contains(got, "beta") || !strings.Contains(got, "Debug") {
		t.Fatalf("output missing beta Debug: %q", got)
	}
}

func TestLogSetLevel_WithExplicitSystems(t *testing.T) {
	var posted []string

	mux := http.NewServeMux()
	mux.HandleFunc("/log/level", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			defer r.Body.Close()
			var req struct {
				Subsystem string `json:"subsystem"`
				Level     string `json:"level"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed decoding request: %v", err)
			}
			if req.Level != "Warn" {
				t.Fatalf("unexpected level: %s", req.Level)
			}
			posted = append(posted, req.Subsystem)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			// Not expected in this test, but keep handler simple
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"levels":{}}`))
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	restore := setTestNodeURL(t, ts)
	defer restore()

	var out bytes.Buffer
	Cmd.SetOut(&out)
	Cmd.SetErr(&out)
	Cmd.SetArgs([]string{"log", "set-level", "Warn", "--system", "alpha", "--system", "beta"})

	if err := Cmd.Execute(); err != nil {
		t.Fatalf("set-level failed: %v", err)
	}

	sort.Strings(posted)
	if len(posted) != 2 || posted[0] != "alpha" || posted[1] != "beta" {
		t.Fatalf("unexpected POSTed subsystems: %v", posted)
	}
}

func TestLogSetLevel_AllSystemsWhenNoneSpecified(t *testing.T) {
	var posted []string

	mux := http.NewServeMux()
	mux.HandleFunc("/log/subsystems", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		_, _ = w.Write([]byte(`{"subsystems":["alpha","beta","gamma"]}`))
	})
	mux.HandleFunc("/log/level", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			defer r.Body.Close()
			var req struct {
				Subsystem string `json:"subsystem"`
				Level     string `json:"level"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed decoding request: %v", err)
			}
			if req.Level != "Error" {
				t.Fatalf("unexpected level: %s", req.Level)
			}
			posted = append(posted, req.Subsystem)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"levels":{}}`))
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	restore := setTestNodeURL(t, ts)
	defer restore()

	var out bytes.Buffer
	Cmd.SetOut(&out)
	Cmd.SetErr(&out)

	// Ensure any prior test-provided --system values are cleared explicitly on the underlying command
	var logCmd, setLevelCmd *cobra.Command
	for _, c := range Cmd.Commands() {
		if c.Use == "log" {
			logCmd = c
			break
		}
	}
	if logCmd == nil {
		t.Fatalf("log command not found")
	}
	for _, c := range logCmd.Commands() {
		if strings.HasPrefix(c.Use, "set-level") {
			setLevelCmd = c
			break
		}
	}
	if setLevelCmd == nil {
		t.Fatalf("set-level command not found")
	}
	// Reset the slice flag to empty using pflag's SliceValue API to avoid state leakage across tests
	if f := setLevelCmd.Flags().Lookup("system"); f != nil {
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace([]string{})
		}
	}

	Cmd.SetArgs([]string{"log", "set-level", "Error"})

	if err := Cmd.Execute(); err != nil {
		t.Fatalf("set-level failed: %v", err)
	}

	sort.Strings(posted)
	expected := []string{"alpha", "beta", "gamma"}
	sort.Strings(expected)
	if strings.Join(posted, ",") != strings.Join(expected, ",") {
		t.Fatalf("unexpected POSTed subsystems: %v (expected %v)", posted, expected)
	}
}
