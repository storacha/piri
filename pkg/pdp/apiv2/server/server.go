package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"

	"github.com/storacha/piri/pkg/pdp/api/middleware"
	"github.com/storacha/piri/pkg/pdp/apiv2"
)

var log = logging.Logger("pdp/server")

// customErrorHandler provides enhanced error handling for ContextualError types
func customErrorHandler(err error, c echo.Context) {
	// Let our middleware handle the error type and logging
	middleware.HandleError(err, c)
}

type Server struct {
	e *echo.Echo
	h *apiv2.API
}

func NewServer(h *apiv2.API) *Server {
	e := echo.New()
	// don't print echo stuff when we start, our logs cover this.
	e.HideBanner = true
	e.HidePort = true

	// handle panics
	e.Use(echomiddleware.Recover())
	// log requests with our logging system
	e.Use(middleware.LogMiddleware(log))

	// Custom error handler for our ContextualError type
	e.HTTPErrorHandler = customErrorHandler

	s := &Server{e: e, h: h}
	registerRoutes(e, s)
	return s
}

const (
	PDPRoutePath     = "/pdp"
	PRoofSetRoutPath = "/proof-sets"
	PiecePrefix      = "/piece"
)

func registerRoutes(e *echo.Echo, s *Server) {
	// /pdp/proof-sets
	proofSets := e.Group(path.Join(PDPRoutePath, PRoofSetRoutPath))
	proofSets.POST("", s.createProofSet)
	proofSets.GET("/created/:txHash", s.getProofSetCreationStatus)

	// /pdp/proof-sets/:proofSetID
	proofSets.GET("/:proofSetID", s.getProofSet)
	proofSets.DELETE("/:proofSetID", s.deleteProofSet)

	// /pdp/proof-sets/:proofSetID/roots
	roots := proofSets.Group("/:proofSetID/roots")
	roots.POST("", s.addRootToProofSet)
	roots.GET("/:rootID", s.getProofSetRoot)
	roots.DELETE("/:rootID", s.deleteProofSetRoot)

	// /pdp/ping
	e.GET("/pdp/ping", s.ping)

	// /pdp/piece
	e.POST(path.Join(PDPRoutePath, piecePrefix), s.addPiece)
	e.PUT(path.Join(PDPRoutePath, piecePrefix, "/upload/:uploadUUID"), s.uploadPiece)
	e.GET(path.Join(PDPRoutePath, piecePrefix), s.findPiece)

	// retrival
	e.GET(path.Join(PiecePrefix, ":cid"), s.getPiece)
}

func (s *Server) Start(ctx context.Context, addr string) error {
	errCh := make(chan error)
	go func() {
		errCh <- s.e.Start(addr)
	}()
	// wait up to one second for the server to start
	// gripe: wish `Start` behaved like a normal start method and didn't block, Run would be a better name. shakes fist at clouds.
	return waitForServerStart(ctx, s.e, errCh, time.Second)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.e.Shutdown(ctx)
}

func waitForServerStart(ctx context.Context, e *echo.Echo, errChan <-chan error, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			var addr net.Addr
			addr = e.ListenerAddr()
			if addr != nil && strings.Contains(addr.String(), ":") {
				return nil // was started
			}
		case err := <-errChan:
			return err
		}
	}
}

func (s *Server) ping(c echo.Context) error {
	operation := "Ping"
	if err := s.h.Ping(c.Request().Context()); err != nil {
		// Extract HTTP status code from the error
		return middleware.FromAPIError(operation, err)
	}
	return c.NoContent(http.StatusOK)
}

func (s *Server) createProofSet(c echo.Context) error {
	ctx := c.Request().Context()
	operation := "CreateProofSet"

	var req apiv2.CreateProofSet
	if err := c.Bind(&req); err != nil {
		return middleware.NewError(operation, "Invalid request body", err, http.StatusBadRequest)
	}

	log.Debugw("Processing CreateProofSet request", "recordKeeper", req.RecordKeeper)

	ref, err := s.h.CreateProofSet(ctx, req)
	if err != nil {
		// Extract HTTP status code from the error
		return middleware.FromAPIError(operation, err)
	}

	// The ref.URL contains the transaction hash
	location := path.Join("/pdp/proof-sets/created", ref.URL)
	c.Response().Header().Set("Location", location)

	log.Infow("Successfully initiated proof set creation", "txHash", ref.URL, "location", location)
	return c.JSON(http.StatusCreated, ref)
}

func (s *Server) deleteProofSet(c echo.Context) error {
	return c.NoContent(http.StatusNotImplemented)
}

// TODO do better, parse it using standard practice way
const piecePrefix = "/piece/"

func (s *Server) getPiece(c echo.Context) error {
	ctx := c.Request().Context()
	operation := "GetPiece"

	// TODO do this instead is it should be equivalent
	//pieceCidStr := path.Base(c.Request().URL.Path)

	// Remove the path up to the piece cid
	prefixLen := len(piecePrefix)
	if len(c.Request().URL.Path) <= prefixLen {
		return middleware.NewError(operation, "path is missing piece CID", fmt.Errorf("invalid request URL"), http.StatusBadRequest)
	}

	pieceCidStr := c.Request().URL.Path[prefixLen:]
	pieceCid, err := cid.Parse(pieceCidStr)
	if err != nil {
		return middleware.NewError(operation, "failed to parse pieceCid", err, http.StatusBadRequest)
	}

	obj, err := s.h.GetPiece(ctx, pieceCidStr)
	if err != nil {
		return middleware.FromAPIError(operation, err)
	}

	bodyReadSeeker, err := makeReadSeeker(obj.Data)
	if err != nil {
		return middleware.NewError(operation, "failed to make body readSeeker", err, http.StatusInternalServerError)
	}
	setHeaders(c.Response(), pieceCid)
	serveContent(c.Response(), c.Request(), abi.UnpaddedPieceSize(obj.Size), bodyReadSeeker)
	return nil
}

func (s *Server) findPiece(c echo.Context) error {
	ctx := c.Request().Context()
	operation := "FindPiece"

	sizeStr := c.QueryParam("size")
	if sizeStr == "" {
		return middleware.NewError(operation, "piece size required", fmt.Errorf("missing size"), http.StatusBadRequest)
	}
	name := c.QueryParam("name")
	if name == "" {
		return middleware.NewError(operation, "piece name required", fmt.Errorf("missing name"), http.StatusBadRequest)
	}
	hash := c.QueryParam("hash")
	if hash == "" {
		return middleware.NewError(operation, "piece hash required", fmt.Errorf("missing hash"), http.StatusBadRequest)
	}

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return middleware.NewError(operation, "failed to parse piece size", err, http.StatusBadRequest)
	}

	// Verify that a 'parked_pieces' entry exists for the given 'piece_cid'
	resp, err := s.h.FindPiece(ctx, apiv2.PieceHash{
		Name: name,
		Hash: hash,
		Size: size,
	})
	if err != nil {
		return middleware.FromAPIError(operation, err)
	}

	return c.JSON(http.StatusOK, resp)
}

func (s *Server) getProofSet(c echo.Context) error {
	ctx := c.Request().Context()
	operation := "GetProofSet"
	proofSetIDStr := c.Param("proofSetID")

	if proofSetIDStr == "" {
		return middleware.NewError(operation, "proofSetID required", fmt.Errorf("missing proofSetID"), http.StatusBadRequest)
	}

	id, err := strconv.ParseUint(proofSetIDStr, 10, 64)
	if err != nil {
		return middleware.NewError(operation, "failed to parse proofSetID", err, http.StatusBadRequest)
	}

	resp, err := s.h.GetProofSet(ctx, id)
	if err != nil {
		return middleware.FromAPIError(operation, err)
	}

	return c.JSON(http.StatusOK, resp)
}

func (s *Server) getProofSetRoot(c echo.Context) error {
	return c.NoContent(http.StatusNotImplemented)
}

func (s *Server) addPiece(c echo.Context) error {
	ctx := c.Request().Context()
	operation := "PreparePiece"

	var req apiv2.AddPiece
	if err := c.Bind(&req); err != nil {
		return middleware.NewError(operation, "Invalid request body", err, http.StatusBadRequest)
	}

	log.Debugw("Processing prepare piece request",
		"name", req.Check,
		"hash", req.Check.Hash,
		"size", req.Check.Size)

	resp, err := s.h.AddPiece(ctx, req)
	if err != nil {
		return middleware.FromAPIError(operation, err)
	}

	if resp == nil {
		return c.NoContent(http.StatusNoContent)
	}

	c.Response().Header().Set(echo.HeaderLocation, resp.URL)
	return c.JSON(http.StatusCreated, resp)
}

func (s *Server) deleteProofSetRoot(c echo.Context) error {
	ctx := c.Request().Context()
	operation := "DeleteProofSetRoot"
	// Step 2: Extract parameters from the URL
	proofSetIdStr := c.Param("proofSetID")
	if proofSetIdStr == "" {
		return middleware.NewError(operation, "proofSetID required", fmt.Errorf("missing proofSetID"), http.StatusBadRequest)
	}
	rootIdStr := c.Param("rootID")
	if rootIdStr == "" {
		return middleware.NewError(operation, "rootID required", fmt.Errorf("missing rootID"), http.StatusBadRequest)
	}

	proofSetID, err := strconv.ParseUint(proofSetIdStr, 10, 64)
	if err != nil {
		return middleware.NewError(operation, "failed to parse proofSetID", err, http.StatusBadRequest)
	}
	rootID, err := strconv.ParseUint(rootIdStr, 10, 64)
	if err != nil {
		return middleware.NewError(operation, "failed to parse rootID", err, http.StatusBadRequest)
	}

	// check if the proofset belongs to the service in pdp_proof_sets
	if err := s.h.DeleteProofSetRoot(ctx, proofSetID, rootID); err != nil {
		return middleware.FromAPIError(operation, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) getProofSetCreationStatus(c echo.Context) error {
	ctx := c.Request().Context()
	operation := "GetProofSetCreationStatus"
	txHash := c.Param("txHash")
	if txHash == "" {
		return middleware.NewError(operation, "txHash required", fmt.Errorf("missing txHash"), http.StatusBadRequest)
	}

	resp, err := s.h.ProofSetCreationStatus(ctx, apiv2.StatusRef{URL: txHash})
	if err != nil {
		return middleware.FromAPIError(operation, err)
	}

	return c.JSON(http.StatusOK, resp)
}

func (s *Server) uploadPiece(c echo.Context) error {
	ctx := c.Request().Context()
	operation := "UploadPiece"
	uploadRef := c.Param("uploadUUID")
	if uploadRef == "" {
		return middleware.NewError(operation, "uploadUUID required", fmt.Errorf("missing uploadUUID"), http.StatusBadRequest)
	}

	log.Debugw("Processing prepare piece request", "uploadRef", uploadRef)
	if err := s.h.UploadPiece(ctx, apiv2.UploadRef{URL: uploadRef}, c.Request().Body); err != nil {
		return middleware.FromAPIError(operation, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func (s *Server) addRootToProofSet(c echo.Context) error {
	ctx := c.Request().Context()
	operation := "AddRootToProofSet"

	proofSetIDStr := c.Param("proofSetID")
	if proofSetIDStr == "" {
		return middleware.NewError(operation, "missing proofSetID", nil, http.StatusBadRequest)
	}

	id, err := strconv.ParseUint(proofSetIDStr, 10, 64)
	if err != nil {
		return middleware.NewError(operation, "invalid proofSetID format", err, http.StatusBadRequest).
			WithContext("proofSetID", proofSetIDStr)
	}

	var req apiv2.AddRootsPayload
	if err := c.Bind(&req); err != nil {
		return middleware.NewError(operation, "failed to parse request body", err, http.StatusBadRequest).
			WithContext("proofSetID", id)
	}

	if err := s.h.AddRootsToProofSet(ctx, id, req.Roots); err != nil {
		return middleware.FromAPIError(operation, err)
	}

	log.Infow("Successfully added roots to proofSet",
		"proofSetID", id,
		"rootCount", len(req.Roots))
	return c.NoContent(http.StatusCreated)
}
