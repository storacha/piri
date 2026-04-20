//go:build e2e

package smelt_tests

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/storacha/smelt/pkg/clients/guppy"
	"github.com/storacha/smelt/pkg/stack"
)

func TestUploadAndRetrieve(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping darwin test")
	}

	tests := []struct {
		name        string
		useS3       bool
		usePostgres bool
	}{
		{
			name: "default",
		},
		{
			name:  "s3",
			useS3: true,
		},
		{
			name:        "postgres",
			usePostgres: true,
		},
		{
			name:        "s3_and_postgres",
			useS3:       true,
			usePostgres: true,
		},
	}

	// Build local piri image from this repo (shared across all subtests)
	localPiri := stack.BuildPiriImage(t, "..")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			s := stack.MustNewStack(
				t,
				stack.WithPiriImage(localPiri),
				stack.WithPiriNodes(stack.PiriNodeConfig{
					Postgres: tt.usePostgres,
					S3:       tt.useS3,
				}),
				stack.WithGuppyImage("ghcr.io/storacha/guppy:main-dev"),
			)

			gup := guppy.MustNewContainerClient(t, s)

			// Login
			err := gup.Login(ctx, "test@example.com", guppy.WithLoginTimeout(10*time.Second))
			if err != nil {
				t.Fatalf("failed to login: %v", err)
			}
			t.Log("Logged in successfully")

			// Create space
			spaceDID, err := gup.GenerateSpace(ctx)
			if err != nil {
				t.Fatalf("failed to generate space: %v", err)
			}
			t.Logf("Created space: %s", spaceDID)

			// Generate test data inside container (10MB)
			dataPath, err := gup.GenerateTestData(ctx, "10MB")
			if err != nil {
				t.Fatalf("failed to generate test data: %v", err)
			}
			t.Logf("Generated test data at: %s", dataPath)

			// Add source and upload
			err = gup.AddSource(ctx, spaceDID, dataPath)
			if err != nil {
				t.Fatalf("failed to add source: %v", err)
			}
			t.Log("Added source")

			cids, err := gup.Upload(ctx, spaceDID, guppy.WithReplicas(1))
			if err != nil {
				t.Fatalf("failed to upload: %v", err)
			}
			if len(cids) == 0 {
				t.Fatal("expected at least one CID from upload")
			}
			t.Logf("Uploaded CIDs: %v", cids)

			dstPath := fmt.Sprintf("/tmp/testdata-download-%d", time.Now().UnixNano())
			err = gup.Retrieve(ctx, spaceDID, cids[len(cids)-1], dstPath)
			require.NoError(t, err)
		})
	}
}
