package spacecontent

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/server/retrieval"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/blobstore"
)

var log = logging.Logger("retrieval/handlers/spacecontent")

func Retrieve(
	ctx context.Context,
	blobs blobstore.BlobGetter,
	inv invocation.Invocation,
	digest multihash.Multihash,
	byteRange *blobstore.Range,
) (result.Result[content.RetrieveOk, failure.IPLDBuilderFailure], retrieval.Response, error) {
	if byteRange == nil {
		return doRetrieve(ctx, blobs, inv, digest)
	}
	return doRangeRetrieve(ctx, blobs, inv, digest, *byteRange)
}

func doRangeRetrieve(
	ctx context.Context,
	blobs blobstore.BlobGetter,
	inv invocation.Invocation,
	digest multihash.Multihash,
	byteRange blobstore.Range,
) (result.Result[content.RetrieveOk, failure.IPLDBuilderFailure], retrieval.Response, error) {
	digestStr := digestutil.Format(digest)
	start := byteRange.Start
	end := byteRange.End
	rangeStr := fmt.Sprintf("%d-", start)
	if end != nil {
		rangeStr += fmt.Sprintf("%d", end)
	}

	cap := inv.Capabilities()[0]
	log := log.With("iss", inv.Issuer().DID(), "can", cap.Can(), "with", cap.With(), "digest", digestStr, "range", rangeStr)

	var getOpts []blobstore.GetOption
	if start > 0 || end != nil {
		getOpts = append(getOpts, blobstore.WithRange(start, end))
	}

	blob, err := blobs.Get(ctx, digest, getOpts...)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			log.Debugw("blob not found", "status", http.StatusNotFound)
			notFoundErr := content.NewNotFoundError(fmt.Sprintf("blob not found: %s", digestStr))
			res := result.Error[content.RetrieveOk, failure.IPLDBuilderFailure](notFoundErr)
			resp := retrieval.NewResponse(http.StatusNotFound, nil, nil)
			return res, resp, nil
		} else if errors.Is(err, blobstore.ErrRangeNotSatisfiable) {
			log.Debugw("range not satisfiable", "status", http.StatusRequestedRangeNotSatisfiable)
			rangeNotSatisfiableErr := content.NewRangeNotSatisfiableError(fmt.Sprintf("range not satisfiable: %d-%d", start, end))
			res := result.Error[content.RetrieveOk, failure.IPLDBuilderFailure](rangeNotSatisfiableErr)
			resp := retrieval.NewResponse(http.StatusRequestedRangeNotSatisfiable, nil, nil)
			return res, resp, nil
		}
		log.Errorw("getting blob", "error", err)
		return nil, retrieval.Response{}, fmt.Errorf("getting blob: %w", err)
	}

	if end == nil {
		rend := uint64(blob.Size() - 1)
		end = &rend
	}

	res := result.Ok[content.RetrieveOk, failure.IPLDBuilderFailure](content.RetrieveOk{})
	status := http.StatusOK
	contentLength := *end - start + 1
	headers := http.Header{}
	headers.Set("Content-Length", fmt.Sprintf("%d", contentLength))
	headers.Set("Content-Type", "application/octet-stream")
	headers.Set("Cache-Control", "public, max-age=29030400, immutable")
	headers.Set("Etag", fmt.Sprintf(`"%s"`, digestStr))
	headers.Set("Vary", "Accept-Encoding")
	if contentLength != uint64(blob.Size()) {
		status = http.StatusPartialContent
		headers.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, *end, blob.Size()))
		headers.Add("Vary", "Range")
	}
	log.Debugw("serving bytes", "status", status, "size", contentLength)
	resp := retrieval.NewResponse(status, headers, blob.Body())
	return res, resp, nil
}

func doRetrieve(
	ctx context.Context,
	blobs blobstore.BlobGetter,
	inv invocation.Invocation,
	digest multihash.Multihash,
) (result.Result[content.RetrieveOk, failure.IPLDBuilderFailure], retrieval.Response, error) {
	digestStr := digestutil.Format(digest)

	cap := inv.Capabilities()[0]
	log := log.With("iss", inv.Issuer().DID(), "can", cap.Can(), "with", cap.With(), "digest", digestStr)

	blob, err := blobs.Get(ctx, digest)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			log.Debugw("blob not found", "status", http.StatusNotFound)
			notFoundErr := content.NewNotFoundError(fmt.Sprintf("blob not found: %s", digestStr))
			res := result.Error[content.RetrieveOk, failure.IPLDBuilderFailure](notFoundErr)
			resp := retrieval.NewResponse(http.StatusNotFound, nil, nil)
			return res, resp, nil
		}
		log.Errorw("getting blob", "error", err)
		return nil, retrieval.Response{}, fmt.Errorf("getting blob: %w", err)
	}

	res := result.Ok[content.RetrieveOk, failure.IPLDBuilderFailure](content.RetrieveOk{})
	status := http.StatusOK
	headers := http.Header{}
	headers.Set("Content-Length", fmt.Sprintf("%d", blob.Size()))
	headers.Set("Content-Type", "application/octet-stream")
	headers.Set("Cache-Control", "public, max-age=29030400, immutable")
	headers.Set("Etag", fmt.Sprintf(`"%s"`, digestStr))
	headers.Set("Vary", "Accept-Encoding")
	log.Debugw("serving bytes", "status", status, "size", blob.Size())
	resp := retrieval.NewResponse(status, headers, blob.Body())
	return res, resp, nil

}
