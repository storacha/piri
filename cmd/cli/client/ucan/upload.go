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
	UploadCmd.Flags().String("space-did", "", "DID for the space to use")
	cobra.CheckErr(UploadCmd.MarkFlagRequired("space-did"))

	UploadCmd.Flags().String("blob", "", "Blob to upload")
	cobra.CheckErr(UploadCmd.MarkFlagRequired("blob"))

	UploadCmd.PersistentFlags().String("proof", "", "CAR file containing storage proof authorizing client invocations")
	cobra.CheckErr(UploadCmd.MarkPersistentFlagRequired("proof"))

}

func doUpload(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load[config.UCANClient]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	c, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	spaceDid, err := did.Parse(cmd.Flag("space-did").Value.String())
	if err != nil {
		return fmt.Errorf("parsing space did: %w", err)
	}
	blobFile, err := os.Open(cmd.Flag("blob").Value.String())
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
	address, err := c.BlobAllocate(cmd.Context(), spaceDid, digest.Bytes(), uint64(len(blobData)), cidlink.Link{Cid: cid.NewCidV1(cid.Raw, digest.Bytes())})
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
	blobResult, err := c.BlobAccept(cmd.Context(), spaceDid, digest.Bytes(), uint64(len(blobData)), cidlink.Link{Cid: cid.NewCidV1(cid.Raw, digest.Bytes())})
	if err != nil {
		return fmt.Errorf("accepting blob: %w", err)
	}
	cmd.Printf("uploaded blob available at: %s\n", blobResult.LocationCommitment.Location[0].String())
	if blobResult.PDPAccept != nil {
		cmd.Printf("submitted for PDP aggregation: %s\n", blobResult.PDPAccept.Piece.Link().String())
	}
	return nil
}
