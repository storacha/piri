package benchmark

import (
	"crypto/rand"
	"fmt"
	"io"
	"time"

	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/minio/sha256-simd"
	"github.com/spf13/cobra"
)

const benchmarkDataSize = 256 * 1024 * 1024 // 256MB

var commpCmd = &cobra.Command{
	Use:   "commp",
	Short: "Benchmark commp.Calc performance",
	Long:  "Generate 256MB of random data and measure the time to calculate commP",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Starting commP benchmark with %d MB of data...\n", benchmarkDataSize/(1024*1024))

		// Create commp calculator
		cp := &commp.Calc{}

		// Start timing
		start := time.Now()

		// Generate and write random data
		limitedReader := io.LimitReader(rand.Reader, benchmarkDataSize)
		written, err := io.Copy(cp, limitedReader)
		if err != nil {
			return fmt.Errorf("failed to write data to commp calculator: %w", err)
		}

		// Calculate digest
		digest, paddedSize, err := cp.Digest()
		if err != nil {
			return fmt.Errorf("failed to compute commP: %w", err)
		}

		// Stop timing
		duration := time.Since(start)

		// Print results
		fmt.Printf("\nBenchmark Results:\n")
		fmt.Printf("Data size: %d bytes (%d MB)\n", written, written/(1024*1024))
		fmt.Printf("Padded piece size: %d bytes\n", paddedSize)
		fmt.Printf("CommP digest: %x\n", digest)
		fmt.Printf("Duration: %v\n", duration)

		// Calculate throughput in MB/s and Gbps
		throughputMBps := float64(written) / (1024 * 1024) / duration.Seconds()
		throughputGbps := float64(written) * 8 / (1000 * 1000 * 1000) / duration.Seconds()

		fmt.Printf("Throughput: %.2f MB/s (%.2f Gbps)\n", throughputMBps, throughputGbps)

		return nil
	},
}

var sha256Cmd = &cobra.Command{
	Use:   "sha256",
	Short: "Benchmark sha256 performance",
	Long:  "Generate 256MB of random data and measure the time to calculate sha256",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Starting commP benchmark with %d MB of data...\n", benchmarkDataSize/(1024*1024))
		// Start timing
		start := time.Now()
		h := sha256.New()

		// Generate and write random data
		limitedReader := io.LimitReader(rand.Reader, benchmarkDataSize)
		written, err := io.Copy(h, limitedReader)
		if err != nil {
			return fmt.Errorf("failed to write data to commp calculator: %w", err)
		}

		// Calculate digest
		digest := h.Sum(nil)

		// Stop timing
		duration := time.Since(start)

		// Print results
		fmt.Printf("\nBenchmark Results:\n")
		fmt.Printf("Data size: %d bytes (%d MB)\n", written, written/(1024*1024))
		fmt.Printf("sha256 digest: %x\n", digest)
		fmt.Printf("Duration: %v\n", duration)

		// Calculate throughput in MB/s and Gbps
		throughputMBps := float64(written) / (1024 * 1024) / duration.Seconds()
		throughputGbps := float64(written) * 8 / (1000 * 1000 * 1000) / duration.Seconds()

		fmt.Printf("Throughput: %.2f MB/s (%.2f Gbps)\n", throughputMBps, throughputGbps)

		return nil
	},
}
