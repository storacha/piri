package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-ucanto/principal"

	"github.com/storacha/piri/pkg/build"
)

var log = logging.Logger("server")

type ServerInfo struct {
	ID    string    `json:"id"`
	Build BuildInfo `json:"build"`
}

type BuildInfo struct {
	Version string `json:"version"`
	Repo    string `json:"repo"`
}

// NewHandler displays version info.
func NewHandler(id principal.Signer) http.Handler {
	info := ServerInfo{
		ID: id.DID().String(),
		Build: BuildInfo{
			Version: build.Version,
			Repo:    "https://github.com/storacha/piri",
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			data, err := json.Marshal(&info)
			if err != nil {
				log.Errorf("failed JSON marshal server info: %w", err)
				http.Error(w, "failed JSON marshal server info", http.StatusInternalServerError)
				return
			}
			w.Write(data)
		} else {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte(fmt.Sprintf("🔥 piri %s\n", info.Build.Version)))
			w.Write([]byte("- https://github.com/storacha/piri\n"))
			w.Write([]byte(fmt.Sprintf("- %s", info.ID)))
		}
	})
}
