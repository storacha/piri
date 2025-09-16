package setup

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/minio/selfupdate"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"github.com/storacha/piri/pkg/build"
	"golang.org/x/mod/semver"
)

func getLatestRelease(ctx context.Context) (*GitHubRelease, error) {
	// Allow overriding the release URL for testing
	releaseURL := ReleaseURL
	if testURL := os.Getenv("PIRI_TEST_GITHUB_API_URL"); testURL != "" {
		releaseURL = testURL + "/repos/storacha/piri/releases/latest"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", releaseURL, nil)
	if err != nil {
		return nil, err
	}

	// GitHub API requires a user agent
	req.Header.Set("User-Agent", "piri-updater")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func findAssetURL(release *GitHubRelease) (string, error) {
	goos := runtime.GOOS
	arch := runtime.GOARCH

	// Look for asset matching our platform
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)

		// Special handling for macOS - look for "mac_os_all.zip"
		if goos == "darwin" {
			if strings.Contains(name, "mac_os_all") && strings.HasSuffix(name, ".zip") {
				return asset.BrowserDownloadURL, nil
			}
			continue
		}

		// For Linux, match architecture-specific tar.gz files
		if goos == "linux" {
			if SupportedLinuxArch[arch] && strings.Contains(name, "linux") &&
				strings.Contains(name, arch) && strings.HasSuffix(name, ".tar.gz") {
				return asset.BrowserDownloadURL, nil
			}
		}
	}

	return "", fmt.Errorf("no suitable release asset found for %s/%s", goos, arch)
}

func downloadAndVerifyBinary(ctx context.Context, cmd *cobra.Command, url string, expectedChecksum []byte, showProgress bool) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	var archiveData []byte
	if showProgress {
		// Create progress bar
		bar := progressbar.NewOptions64(
			resp.ContentLength,
			progressbar.OptionSetWriter(cmd.OutOrStdout()),
			progressbar.OptionEnableColorCodes(true),
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetWidth(30),
			progressbar.OptionSetDescription("Downloading update"),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "[green]=[reset]",
				SaucerHead:    "[green]🐣>[reset]",
				SaucerPadding: " ",
				BarStart:      "🥚",
				BarEnd:        "🐔",
			}),
			progressbar.OptionOnCompletion(func() {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), "\n")
			}),
		)

		// Read the entire archive into memory to verify checksum
		archiveData, err = io.ReadAll(io.TeeReader(resp.Body, bar))
		if err != nil {
			return nil, fmt.Errorf("failed to read archive: %w", err)
		}

		// Ensure the progress bar is finished
		_ = bar.Finish()
	} else {
		archiveData, err = io.ReadAll(resp.Body)
	}

	// Verify checksum if provided
	if expectedChecksum != nil {
		cmd.Println("Verifying archive checksum...")
		hash := sha256.New()
		hash.Write(archiveData)
		actualChecksum := hash.Sum(nil)

		if !bytes.Equal(actualChecksum, expectedChecksum) {
			return nil, fmt.Errorf("archive checksum mismatch: expected %x, got %x",
				expectedChecksum, actualChecksum)
		}
		cmd.Println("Archive checksum verified successfully")
	}

	// Now extract the binary from the verified archive
	archiveReader := io.NopCloser(bytes.NewReader(archiveData))

	// Check file type and extract accordingly
	if strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz") {
		return extractBinaryFromTarGz(cmd, archiveReader)
	} else if strings.HasSuffix(url, ".zip") {
		return extractBinaryFromZip(cmd, archiveReader)
	}

	// If it's not an archive, return the data as-is
	return io.NopCloser(bytes.NewReader(archiveData)), nil
}

// extractBinaryFromTarGz extracts the piri binary from a tar.gz archive
func extractBinaryFromTarGz(cmd *cobra.Command, r io.ReadCloser) (io.ReadCloser, error) {
	defer r.Close()

	// Read the entire response into memory (binaries are typically small)
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read archive: %w", err)
	}

	// Create gzip reader
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gr.Close()

	// Create tar reader
	tr := tar.NewReader(gr)

	// Look for the piri binary in the archive
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}

		// Look for the piri binary (could be just "piri" or in a subdirectory)
		fileName := strings.ToLower(header.Name)
		if strings.HasSuffix(fileName, "piri") || strings.HasSuffix(fileName, "piri.exe") {
			// Check if it's a regular file and executable
			if header.Typeflag == tar.TypeReg {
				// Read the binary content
				binaryData, err := io.ReadAll(tr)
				if err != nil {
					return nil, fmt.Errorf("failed to extract binary: %w", err)
				}

				cmd.Printf("Extracted binary from archive: %s (%d bytes)\n", header.Name, len(binaryData))
				return io.NopCloser(bytes.NewReader(binaryData)), nil
			}
		}
	}

	return nil, fmt.Errorf("piri binary not found in archive")
}

// extractBinaryFromZip extracts the piri binary from a zip archive
func extractBinaryFromZip(cmd *cobra.Command, r io.ReadCloser) (io.ReadCloser, error) {
	defer r.Close()

	// Read the entire response into memory
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read archive: %w", err)
	}

	// Create zip reader
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to create zip reader: %w", err)
	}

	// Look for the piri binary in the archive
	for _, file := range zr.File {
		fileName := strings.ToLower(file.Name)
		// Look for the piri binary
		if strings.HasSuffix(fileName, "piri") || strings.HasSuffix(fileName, "piri.exe") {
			// Open the file in the zip
			rc, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open file in zip: %w", err)
			}

			// Read the binary content
			binaryData, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to extract binary: %w", err)
			}

			cmd.Printf("Extracted binary from zip archive: %s (%d bytes)\n", file.Name, len(binaryData))
			return io.NopCloser(bytes.NewReader(binaryData)), nil
		}
	}

	return nil, fmt.Errorf("piri binary not found in zip archive")
}

// getAssetChecksum downloads the checksums file and extracts the checksum for the given asset
func getAssetChecksum(ctx context.Context, cmd *cobra.Command, release *GitHubRelease, assetFileName string) ([]byte, error) {
	// Find the checksums file in the release assets
	var checksumURL string
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, "checksums.txt") {
			checksumURL = asset.BrowserDownloadURL
			break
		}
	}

	if checksumURL == "" {
		return nil, fmt.Errorf("checksums file not found in release")
	}

	// Download the checksums file
	req, err := http.NewRequestWithContext(ctx, "GET", checksumURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "piri-updater")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download checksums: status %d", resp.StatusCode)
	}

	// Parse the checksums file
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Format: <sha256_hash>  <filename>
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}

		hashHex := parts[0]
		fileName := parts[1]

		// Check if this is the checksum for our asset
		if fileName == assetFileName {
			checksum, err := hex.DecodeString(hashHex)
			if err != nil {
				return nil, fmt.Errorf("invalid checksum format: %w", err)
			}
			cmd.Printf("Found checksum for %s: %s\n", assetFileName, hashHex)
			return checksum, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading checksums file: %w", err)
	}

	return nil, fmt.Errorf("checksum not found for %s", assetFileName)
}

// UpdateInfo contains information about available updates
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	NeedsUpdate    bool
	Release        *GitHubRelease
}

// checkForUpdate checks if an update is available
func checkForUpdate(ctx context.Context, cmd *cobra.Command) (*UpdateInfo, error) {
	currentVersion := build.Version
	cmd.Printf("Current version: %s\n", currentVersion)

	// Check for latest release
	release, err := getLatestRelease(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest release: %w", err)
	}

	// Prepare versions for comparison
	// semver requires "v" prefix, so ensure both have it
	latestVersion := release.TagName
	if !strings.HasPrefix(latestVersion, "v") {
		latestVersion = "v" + latestVersion
	}

	// Clean current version (remove build metadata after "-")
	currentVersionClean := strings.Split(currentVersion, "-")[0]
	if !strings.HasPrefix(currentVersionClean, "v") {
		currentVersionClean = "v" + currentVersionClean
	}

	cmd.Printf("Latest version: %s\n", latestVersion)

	// Use semver to properly compare versions
	// semver.Compare returns -1 if v1 < v2, 0 if v1 == v2, +1 if v1 > v2
	needsUpdate := false
	if semver.IsValid(currentVersionClean) && semver.IsValid(latestVersion) {
		// Only update if latest is greater than current
		needsUpdate = semver.Compare(latestVersion, currentVersionClean) > 0
	} else {
		// Fallback to string comparison if versions aren't valid semver
		cmd.Printf("Warning: Unable to parse versions as semver, using string comparison\n")
		needsUpdate = latestVersion != currentVersionClean
	}

	return &UpdateInfo{
		CurrentVersion: strings.TrimPrefix(currentVersionClean, "v"),
		LatestVersion:  strings.TrimPrefix(latestVersion, "v"),
		NeedsUpdate:    needsUpdate,
		Release:        release,
	}, nil
}

// downloadAndApplyUpdate downloads and applies the update to the binary
// If targetPath is provided, the update is written to that path instead of execPath
// This allows using the same logic for both standalone and managed installations
func downloadAndApplyUpdate(ctx context.Context, cmd *cobra.Command, release *GitHubRelease, execPath string, targetPath string, showProgress bool) error {
	// Find the appropriate asset for this platform
	assetURL, err := findAssetURL(release)
	if err != nil {
		return fmt.Errorf("failed to find appropriate release asset: %w", err)
	}

	cmd.Printf("Downloading update from %s\n", assetURL)

	// Get the filename from the URL
	assetFileName := path.Base(assetURL)

	// Download and parse checksums
	cmd.Println("Fetching checksums...")
	checksum, err := getAssetChecksum(ctx, cmd, release, assetFileName)
	if err != nil {
		return fmt.Errorf("failed to get asset checksum, aborting update: %w", err)
	}

	// Download and verify the archive, then extract the binary
	newBinary, err := downloadAndVerifyBinary(ctx, cmd, assetURL, checksum, showProgress)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer newBinary.Close()

	// Determine the actual target path
	updateTarget := execPath
	oldSavePath := execPath + ".old"

	// If targetPath is provided, use it instead (for managed installations)
	if targetPath != "" {
		updateTarget = targetPath
		oldSavePath = "" // Don't save old versions for managed installs (we keep them in versioned dirs)
	} else {
		// Safety check: Don't allow updating managed installations without explicit targetPath
		if strings.HasPrefix(execPath, PiriOptDir) {
			return fmt.Errorf("cannot update managed installation at %s without explicit target path", execPath)
		}
	}

	// Apply the update
	cmd.Println("Applying update...")

	// Check if target file exists to determine update method
	if _, err := os.Stat(updateTarget); os.IsNotExist(err) {
		// New file (managed installation creating new version) - write directly
		binaryData, err := io.ReadAll(newBinary)
		if err != nil {
			return fmt.Errorf("failed to read update binary: %w", err)
		}
		if err := os.WriteFile(updateTarget, binaryData, 0755); err != nil {
			return fmt.Errorf("failed to write update: %w", err)
		}
	} else {
		// Existing file - use selfupdate for atomic replacement with optional backup
		err = selfupdate.Apply(newBinary, selfupdate.Options{
			TargetPath:  updateTarget,
			OldSavePath: oldSavePath,
		})
		if err != nil {
			if rerr := selfupdate.RollbackError(err); rerr != nil {
				return fmt.Errorf("failed to apply update and rollback: %w", rerr)
			}
			return fmt.Errorf("failed to apply update: %w", err)
		}
	}

	cmd.Printf("Successfully updated to version %s\n", release.TagName)
	return nil
}
