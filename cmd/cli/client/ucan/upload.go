package ucan

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/spf13/cobra"
	"github.com/storacha/go-ucanto/core/ipld/hash/sha256"
	"github.com/storacha/go-ucanto/did"

	"github.com/storacha/piri/cmd/client"
	"github.com/storacha/piri/pkg/config"
)

var (
	UploadCmd = &cobra.Command{
		Use:   "upload",
		Short: "Invoke a blob allocation",
		// TODO the file/blob to upload ought to be the single argument, instead of a flag
		Args: cobra.NoArgs,
		RunE: doUpload,
	}
)

func init() {
	UploadCmd.Flags().String("node-did", "", "DID of a Piri node")
	cobra.CheckErr(UploadCmd.MarkFlagRequired("node-did"))

	UploadCmd.Flags().String("space-did", "", "DID for the space to use")
	cobra.CheckErr(UploadCmd.MarkFlagRequired("space-did"))

	UploadCmd.Flags().String("blob", "", "Blob to upload")
	cobra.CheckErr(UploadCmd.MarkFlagRequired("blob"))

	UploadCmd.Flags().String("proof", "", "CAR file containing storage proof authorizing client invocations")
	cobra.CheckErr(UploadCmd.MarkFlagRequired("proof"))

}

func doUpload(cmd *cobra.Command, _ []string) error {
	// load client config to get identity and endpoint
	cfg, err := config.Load[config.Client]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// grab the flags
	nodeDID, err := cmd.Flags().GetString("node-did")
	if err != nil {
		return fmt.Errorf("failed to get node-did flag: %w", err)
	}
	spaceDIDStr, err := cmd.Flags().GetString("space-did")
	if err != nil {
		return fmt.Errorf("failed to get space-did flag: %w", err)
	}
	spaceDID, err := did.Parse(spaceDIDStr)
	if err != nil {
		return fmt.Errorf("failed to parse space did: %w", err)
	}
	blobPath, err := cmd.Flags().GetString("blob")
	if err != nil {
		return fmt.Errorf("failed to get blob flag: %w", err)
	}
	proof, err := cmd.Flags().GetString("proof")
	if err != nil {
		return fmt.Errorf("failed to get proof flag: %w", err)
	}

	// build a client from flags and config
	c, err := client.New(client.Config{
		KeyFile: cfg.Identity.KeyFile,
		NodeURL: cfg.API.Endpoint,
		Proof:   proof,
		NodeDID: nodeDID,
	})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	blobFile, err := os.Open(blobPath)
	if err != nil {
		return fmt.Errorf("opening blob file: %w", err)
	}
	blobData, err := io.ReadAll(blobFile)
	if err != nil {
		return fmt.Errorf("reading blob file: %w", err)
	}
	digest, err := sha256.Hasher.Sum(blobData)
	if err != nil {
		return fmt.Errorf("calculating blob digest: %w", err)
	}
	address, err := c.BlobAllocate(cmd.Context(), spaceDID, digest.Bytes(), uint64(len(blobData)), cidlink.Link{Cid: cid.NewCidV1(cid.Raw, digest.Bytes())})
	if err != nil {
		return fmt.Errorf("invocing blob allocation: %w", err)
	}
	if address != nil {
		cmd.Printf("now uploading to: %s\n", address.URL.String())

		req, err := http.NewRequest(http.MethodPut, address.URL.String(), bytes.NewReader(blobData))
		if err != nil {
			return fmt.Errorf("uploading blob: %w", err)
		}
		req.Header = address.Headers
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("sending blob: %w", err)
		}
		if res.StatusCode >= 300 || res.StatusCode < 200 {
			resData, err := io.ReadAll(res.Body)
			if err != nil {
				return fmt.Errorf("reading response body: %w", err)
			}
			return fmt.Errorf("unsuccessful put, status: %s, message: %s", res.Status, string(resData))
		}
	}
	blobResult, err := c.BlobAccept(cmd.Context(), spaceDID, digest.Bytes(), uint64(len(blobData)), cidlink.Link{Cid: cid.NewCidV1(cid.Raw, digest.Bytes())})
	if err != nil {
		return fmt.Errorf("accepting blob: %w", err)
	}
	cmd.Printf("uploaded blob available at: %s\n", blobResult.LocationCommitment.Location[0].String())
	if blobResult.PDPAccept != nil {
		cmd.Printf("submitted for PDP aggregation: %s\n", blobResult.PDPAccept.Piece.Link().String())
	}
	return nil
}
