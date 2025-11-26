package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultMinFileSize = int64(256 * 1024)       // 256 KiB keeps file counts modest
const defaultMaxFileSize = int64(32 * 1024 * 1024) // 32 MiB files by default to reduce syscall overhead

type generator struct {
	rng         *rand.Rand
	root        string
	remaining   int64
	written     int64
	fileCount   int
	dirCount    int
	minFileSize int64
	maxFileSize int64
	buf         []byte
}

func main() {
	outputDir := flag.String("output", "./random-dir", "Directory to create and populate with random content")
	sizeFlag := flag.String("size", "10MB", "Total size of data to generate (e.g. 512KB, 10MB, 1GB)")
	seedFlag := flag.Int64("seed", time.Now().UnixNano(), "Seed for deterministic generation; same seed yields the same layout and content")
	minFileFlag := flag.String("min-file-size", "256KB", "Minimum size per file; smaller sizes create more files (e.g. 64KB, 1MB)")
	maxFileFlag := flag.String("max-file-size", "32MB", "Maximum size per file; larger sizes generate fewer files and are faster (e.g. 8MB, 64MB)")
	flag.Parse()

	targetBytes, err := parseByteSize(*sizeFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid size %q: %v\n", *sizeFlag, err)
		os.Exit(1)
	}
	if targetBytes <= 0 {
		fmt.Fprintln(os.Stderr, "size must be greater than zero")
		os.Exit(1)
	}

	maxFileSize, err := parseByteSize(*maxFileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid max-file-size %q: %v\n", *maxFileFlag, err)
		os.Exit(1)
	}
	if maxFileSize <= 0 {
		fmt.Fprintln(os.Stderr, "max-file-size must be greater than zero")
		os.Exit(1)
	}

	minFileSize, err := parseByteSize(*minFileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid min-file-size %q: %v\n", *minFileFlag, err)
		os.Exit(1)
	}
	if minFileSize <= 0 {
		fmt.Fprintln(os.Stderr, "min-file-size must be greater than zero")
		os.Exit(1)
	}
	if minFileSize < defaultMinFileSize {
		// Clamp to avoid millions of tiny files by accident.
		minFileSize = defaultMinFileSize
	}
	if minFileSize > maxFileSize {
		fmt.Fprintf(os.Stderr, "min-file-size (%d bytes) cannot exceed max-file-size (%d bytes)\n", minFileSize, maxFileSize)
		os.Exit(1)
	}

	gen := &generator{
		rng:         rand.New(rand.NewSource(*seedFlag)),
		root:        *outputDir,
		remaining:   targetBytes,
		minFileSize: minFileSize,
		maxFileSize: maxFileSize,
		buf:         make([]byte, chooseBufferSize(maxFileSize)),
	}
	if err := gen.Generate(); err != nil {
		fmt.Fprintf(os.Stderr, "generation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %d bytes in %s using seed %d\n", gen.written, gen.root, *seedFlag)
}

// Generate constructs a reproducible directory tree containing both files and subdirectories with files.
func (g *generator) Generate() error {
	if g.remaining < 2 {
		return errors.New("size too small to place files in both root and a subdirectory")
	}
	if err := os.MkdirAll(g.root, 0o755); err != nil {
		return err
	}

	// Always place one file at the root and another inside a subdirectory to satisfy the structure requirement.
	rootFileSize := g.takeFileSize(g.remaining / 2)
	if err := g.writeFile(g.root, rootFileSize); err != nil {
		return err
	}

	subdirPath, err := g.makeSubdir(g.root)
	if err != nil {
		return err
	}
	if err := g.writeFile(subdirPath, g.takeFileSize(g.remaining)); err != nil {
		return err
	}

	directories := []string{g.root, subdirPath}

	for g.remaining > 0 {
		currentDir := directories[g.rng.Intn(len(directories))]

		// Occasionally branch deeper, biased by remaining space.
		if g.remaining > g.minFileSize*2 && g.rng.Float64() < 0.35 {
			newDir, err := g.makeSubdir(currentDir)
			if err != nil {
				return err
			}
			directories = append(directories, newDir)
			continue
		}

		if err := g.writeFile(currentDir, g.takeFileSize(g.remaining)); err != nil {
			return err
		}
	}

	return nil
}

func (g *generator) writeFile(dir string, size int64) error {
	if size <= 0 {
		return nil
	}

	g.fileCount++
	fileName := fmt.Sprintf("file_%03d_%s.bin", g.fileCount, g.randToken(4))
	fullPath := filepath.Join(dir, fileName)

	f, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriterSize(f, len(g.buf))
	remaining := size
	for remaining > 0 {
		chunk := int64(len(g.buf))
		if chunk > remaining {
			chunk = remaining
		}
		if _, err := g.rng.Read(g.buf[:chunk]); err != nil {
			return err
		}
		if _, err := w.Write(g.buf[:chunk]); err != nil {
			return err
		}
		remaining -= chunk
	}
	if err := w.Flush(); err != nil {
		return err
	}

	g.remaining -= size
	g.written += size
	return nil
}

func (g *generator) makeSubdir(parent string) (string, error) {
	g.dirCount++
	dirName := fmt.Sprintf("dir_%03d_%s", g.dirCount, g.randToken(3))
	path := filepath.Join(parent, dirName)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func (g *generator) takeFileSize(maxBytes int64) int64 {
	if maxBytes <= 0 {
		return 0
	}
	maxCandidate := maxBytes
	if maxCandidate > g.maxFileSize {
		maxCandidate = g.maxFileSize
	}

	minCandidate := g.minFileSize
	if minCandidate > maxCandidate {
		minCandidate = maxCandidate
	}

	size := minCandidate
	if maxCandidate > minCandidate {
		size = g.rng.Int63n(maxCandidate-minCandidate+1) + minCandidate
	}
	return size
}

func (g *generator) randToken(length int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	out := make([]byte, length)
	for i := 0; i < length; i++ {
		out[i] = letters[g.rng.Intn(len(letters))]
	}
	return string(out)
}

func parseByteSize(value string) (int64, error) {
	clean := strings.TrimSpace(strings.ToUpper(value))
	if clean == "" {
		return 0, errors.New("empty size value")
	}

	var numberPart strings.Builder
	var unitPart strings.Builder
	for _, r := range clean {
		switch {
		case (r >= '0' && r <= '9') || r == '.':
			numberPart.WriteRune(r)
		case r == '_' || r == ',':
			continue
		default:
			unitPart.WriteRune(r)
		}
	}

	if numberPart.Len() == 0 {
		return 0, errors.New("missing numeric portion")
	}

	parsedNum, err := strconv.ParseFloat(numberPart.String(), 64)
	if err != nil {
		return 0, err
	}

	unit := unitPart.String()
	multiplier := int64(1)
	switch unit {
	case "", "B":
		multiplier = 1
	case "K", "KB":
		multiplier = 1 << 10
	case "M", "MB":
		multiplier = 1 << 20
	case "G", "GB":
		multiplier = 1 << 30
	case "T", "TB":
		multiplier = 1 << 40
	default:
		return 0, fmt.Errorf("unknown size unit %q", unit)
	}

	result := int64(parsedNum * float64(multiplier))
	if result < 0 {
		return 0, errors.New("size must be positive")
	}
	return result, nil
}

func chooseBufferSize(maxFileSize int64) int {
	switch {
	case maxFileSize >= 64*1024*1024:
		return 4 * 1024 * 1024
	case maxFileSize >= 8*1024*1024:
		return 2 * 1024 * 1024
	default:
		return 1 * 1024 * 1024
	}
}
