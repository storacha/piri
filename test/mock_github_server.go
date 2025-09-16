package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// GitHubRelease mimics the GitHub API release structure
type GitHubRelease struct {
	TagName    string    `json:"tag_name"`
	Name       string    `json:"name"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	CreatedAt  time.Time `json:"created_at"`
	Assets     []Asset   `json:"assets"`
}

// Asset represents a release asset
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Config holds server configuration
type Config struct {
	BinaryPath        string
	AdvertisedVersion string
	Port              int
	ServerURL         string
}

// Server holds the mock server state
type Server struct {
	config    Config
	archives  map[string][]byte // arch -> tar.gz bytes
	checksums map[string]string // filename -> sha256
}

func main() {
	var config Config
	flag.StringVar(&config.BinaryPath, "binary-path", "", "Path to piri binary to serve (required)")
	flag.StringVar(&config.AdvertisedVersion, "advertised-version", "v99.99.99", "Version to advertise in API response")
	flag.IntVar(&config.Port, "port", 8080, "Server port")
	flag.Parse()

	if config.BinaryPath == "" {
		log.Fatal("--binary-path is required")
	}

	// Verify binary exists
	if _, err := os.Stat(config.BinaryPath); err != nil {
		log.Fatalf("Binary not found at %s: %v", config.BinaryPath, err)
	}

	config.ServerURL = fmt.Sprintf("http://localhost:%d", config.Port)

	server := &Server{
		config:    config,
		archives:  make(map[string][]byte),
		checksums: make(map[string]string),
	}

	// Prepare archives for both architectures
	if err := server.prepareArchives(); err != nil {
		log.Fatalf("Failed to prepare archives: %v", err)
	}

	// Setup routes
	http.HandleFunc("/repos/storacha/piri/releases/latest", server.handleLatestRelease)
	http.HandleFunc("/download/", server.handleDownload)

	log.Printf("Mock GitHub API server starting on port %d", config.Port)
	log.Printf("Serving binary: %s", config.BinaryPath)
	log.Printf("Advertising version: %s", config.AdvertisedVersion)
	log.Printf("API endpoint: %s/repos/storacha/piri/releases/latest", config.ServerURL)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", config.Port), nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// prepareArchives creates tar.gz archives for each architecture
func (s *Server) prepareArchives() error {
	architectures := []string{"amd64", "arm64"}

	// Read the binary once
	binaryData, err := os.ReadFile(s.config.BinaryPath)
	if err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}

	for _, arch := range architectures {
		archiveName := fmt.Sprintf("piri_linux_%s.tar.gz", arch)

		// Create tar.gz in memory
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)

		// Add the binary to the archive
		header := &tar.Header{
			Name:    "piri",
			Mode:    0755,
			Size:    int64(len(binaryData)),
			ModTime: time.Now(),
		}

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		if _, err := tw.Write(binaryData); err != nil {
			return fmt.Errorf("failed to write binary to tar: %w", err)
		}

		// Close writers
		if err := tw.Close(); err != nil {
			return fmt.Errorf("failed to close tar writer: %w", err)
		}
		if err := gw.Close(); err != nil {
			return fmt.Errorf("failed to close gzip writer: %w", err)
		}

		archiveData := buf.Bytes()
		s.archives[arch] = archiveData

		// Calculate SHA256
		hash := sha256.Sum256(archiveData)
		s.checksums[archiveName] = hex.EncodeToString(hash[:])

		log.Printf("Prepared %s (%d bytes, SHA256: %s)", archiveName, len(archiveData), s.checksums[archiveName])
	}

	return nil
}

// handleLatestRelease responds with mock GitHub release JSON
func (s *Server) handleLatestRelease(w http.ResponseWriter, r *http.Request) {
	log.Printf("GET %s from %s", r.URL.Path, r.RemoteAddr)

	// Build assets list
	assets := []Asset{
		{
			Name:               "piri_linux_amd64.tar.gz",
			BrowserDownloadURL: fmt.Sprintf("%s/download/piri_linux_amd64.tar.gz", s.config.ServerURL),
		},
		{
			Name:               "piri_linux_arm64.tar.gz",
			BrowserDownloadURL: fmt.Sprintf("%s/download/piri_linux_arm64.tar.gz", s.config.ServerURL),
		},
		{
			Name:               "checksums.txt",
			BrowserDownloadURL: fmt.Sprintf("%s/download/checksums.txt", s.config.ServerURL),
		},
	}

	release := GitHubRelease{
		TagName:    s.config.AdvertisedVersion,
		Name:       fmt.Sprintf("Release %s", s.config.AdvertisedVersion),
		Draft:      false,
		Prerelease: false,
		CreatedAt:  time.Now(),
		Assets:     assets,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(release); err != nil {
		log.Printf("Failed to encode response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleDownload serves the tar.gz archives and checksums
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	filename := filepath.Base(r.URL.Path)
	log.Printf("GET %s from %s", filename, r.RemoteAddr)

	switch filename {
	case "piri_linux_amd64.tar.gz":
		w.Header().Set("Content-Type", "application/gzip")
		w.Write(s.archives["amd64"])

	case "piri_linux_arm64.tar.gz":
		w.Header().Set("Content-Type", "application/gzip")
		w.Write(s.archives["arm64"])

	case "checksums.txt":
		w.Header().Set("Content-Type", "text/plain")
		// Write checksums in the format expected by piri
		for filename, checksum := range s.checksums {
			fmt.Fprintf(w, "%s  %s\n", checksum, filename)
		}

	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}