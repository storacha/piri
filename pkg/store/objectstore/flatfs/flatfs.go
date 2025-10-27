// Package flatfs is a Datastore implementation that stores all
// objects in a two-level directory structure in the local file
// system, regardless of the hierarchy of the keys.
package flatfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/piri/pkg/store/objectstore"
)

var log = logging.Logger("flatfs")

const extension = ".data"

var (
	// RetryDelay is a timeout for a backoff on retrying operations
	// that fail due to transient errors like too many file descriptors open.
	RetryDelay = time.Millisecond * 200
	// RetryAttempts is the maximum number of retries that will be attempted
	// before giving up.
	RetryAttempts = 6
)

const (
	opPut = iota
	opDelete
)

var _ objectstore.Store = (*Store)(nil)

var (
	ErrStoreExists         = errors.New("datastore already exists")
	ErrStoreDoesNotExist   = errors.New("datastore directory does not exist")
	ErrShardingFileMissing = fmt.Errorf("%s file not found in datastore", SHARDING_FN)
	ErrClosed              = errors.New("datastore closed")
	ErrInvalidKey          = errors.New("key not supported by flatfs")
	// ErrRangeNotSatisfiable is returned when the byte range option falls outside
	// of the total size of the blob.
	ErrRangeNotSatisfiable = errors.New("range not satisfiable")
)

// Store implements [objectstore.Store].
// Note this datastore cannot guarantee order of concurrent
// write operations to the same key. See the explanation in
// Put().
type Store struct {
	path     string
	tempPath string

	shardStr string
	getDir   ShardFunc

	// synchronize all writes and directory changes for added safety
	sync bool

	shutdownLock sync.RWMutex
	shutdown     bool

	// opMap handles concurrent write operations (put/delete)
	// to the same key
	opMap *opMap
}

type ShardFunc func(string) string

type opT int

// op wraps useful arguments of write operations
type op struct {
	typ  opT       // operation type
	key  string    // datastore key. Mandatory.
	tmp  string    // temp file path
	path string    // file path
	size uint64    // value size in bytes
	v    io.Reader // value
}

// opMap is a synchronisation structure where a single op can be stored
// for each key.
type opMap struct {
	ops sync.Map
}

type opResult struct {
	mu      sync.RWMutex
	success bool

	opMap *opMap
	name  string
}

// Begins starts the processing of an op:
// - if no other op for the same key exist, register it and return immediately
// - if another op exist for the same key, wait until it's done:
//   - if that previous op succeeded, consider that ours shouldn't execute and return nil
//   - if that previous op failed, start ours
func (m *opMap) Begin(name string) *opResult {
	for {
		myOp := &opResult{opMap: m, name: name}
		myOp.mu.Lock()
		opIface, loaded := m.ops.LoadOrStore(name, myOp)
		if !loaded { // no one else doing ops with this key
			return myOp
		}

		op := opIface.(*opResult)
		// someone else doing ops with this key, wait for
		// the result
		op.mu.RLock()
		if op.success {
			return nil
		}

		// if we are here, we will retry the operation
	}
}

func (o *opResult) Finish(ok bool) {
	o.success = ok
	o.opMap.ops.Delete(o.name)
	o.mu.Unlock()
}

func create(path string, fun *ShardIdV1) error {
	err := os.Mkdir(path, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	dsFun, err := ReadShardFunc(path)
	switch err {
	case ErrShardingFileMissing:
		isEmpty, err := DirIsEmpty(path)
		if err != nil {
			return err
		}
		if !isEmpty {
			return fmt.Errorf("directory missing %s file: %s", SHARDING_FN, path)
		}

		err = WriteShardFunc(path, fun)
		if err != nil {
			return err
		}
		err = WriteReadme(path, fun)
		return err
	case nil:
		if fun.String() != dsFun.String() {
			return fmt.Errorf("specified shard func '%s' does not match repo shard func '%s'",
				fun.String(), dsFun.String())
		}
		return ErrStoreExists
	default:
		return err
	}
}

func open(path string, syncFiles bool) (*Store, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, ErrStoreDoesNotExist
	} else if err != nil {
		return nil, err
	}

	tempPath := filepath.Join(path, ".temp")
	err = os.RemoveAll(tempPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to remove temporary directory: %v", err)
	}

	err = os.Mkdir(tempPath, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %v", err)
	}

	shardId, err := ReadShardFunc(path)
	if err != nil {
		return nil, err
	}

	fs := &Store{
		path:     path,
		tempPath: tempPath,
		shardStr: shardId.String(),
		getDir:   shardId.Func(),
		sync:     syncFiles,
		opMap:    new(opMap),
	}

	return fs, nil
}

// New creates a new FlatFS object store or opens an existing one.
func New(path string, fun *ShardIdV1, sync bool) (*Store, error) {
	err := create(path, fun)
	if err != nil && err != ErrStoreExists {
		return nil, err
	}
	return open(path, sync)
}

func (fs *Store) ShardStr() string {
	return fs.shardStr
}

// encode returns the directory and file names for a given key according to
// the sharding function.
func (fs *Store) encode(key string) (dir, file string) {
	dir = filepath.Join(fs.path, fs.getDir(key))
	file = filepath.Join(dir, key+extension)
	return dir, file
}

// makeDir is identical to makeDirNoSync but also enforce the sync
// if required by the config.
func (fs *Store) makeDir(dir string) error {
	created, err := fs.makeDirNoSync(dir)
	if err != nil {
		return err
	}

	// In theory, if we create a new prefix dir and add a file to
	// it, the creation of the prefix dir itself might not be
	// durable yet. Sync the root dir after a successful mkdir of
	// a prefix dir, just to be paranoid.
	if fs.sync && created {
		if err := syncDir(fs.path); err != nil {
			return err
		}
	}
	return nil
}

// makeDirNoSync create a directory on disk and report if it was created or
// already existed.
func (fs *Store) makeDirNoSync(dir string) (created bool, err error) {
	if err := os.Mkdir(dir, 0755); err != nil {
		if os.IsExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// This function always runs under an opLock. Therefore, only one thread is
// touching the affected files.
func (fs *Store) rename(tmpPath, path string) error {
	var err error
	for i := 0; i < RetryAttempts; i++ {
		err = os.Rename(tmpPath, path)
		// if there's no error, or the source file doesn't exist, abort.
		if err == nil || os.IsNotExist(err) {
			break
		}
		// Otherwise, this could be a transient error due to some other
		// process holding open one of the files. Wait a bit and then
		// retry.
		time.Sleep(time.Duration(i+1) * RetryDelay)
	}
	return err
}

// Put stores a key/value in the datastore.
//
// Note, that we do not guarantee order of write operations (Put or Delete)
// to the same key in this datastore.
//
// For example. i.e. in the case of two concurrent Put, we only guarantee
// that one of them will come through, but cannot assure which one even if
// one arrived slightly later than the other. In the case of a
// concurrent Put and a Delete operation, we cannot guarantee which one
// will win.
func (fs *Store) Put(ctx context.Context, key string, size uint64, value io.Reader) error {
	if !keyIsValid(key) {
		return fmt.Errorf("when putting %q: %v", key, ErrInvalidKey)
	}

	fs.shutdownLock.RLock()
	defer fs.shutdownLock.RUnlock()
	if fs.shutdown {
		return ErrClosed
	}

	_, err := fs.doWriteOp(&op{
		typ:  opPut,
		key:  key,
		v:    value,
		size: size,
	})
	return err
}

func (fs *Store) doOp(oper *op) error {
	switch oper.typ {
	case opPut:
		return fs.doPut(oper.key, oper.size, oper.v)
	case opDelete:
		return fs.doDelete(oper.key)
	default:
		panic("bad operation, this is a bug")
	}
}

func isTooManyFDError(err error) bool {
	perr, ok := err.(*os.PathError)
	if ok && perr.Err == syscall.EMFILE {
		return true
	}
	return false
}

// doWrite optimizes out write operations (put/delete) to the same
// key by queueing them and succeeding all queued
// operations if one of them does. In such case,
// we assume that the first succeeding operation
// on that key was the last one to happen after
// all successful others.
//
// done is true if we actually performed the operation, false if we skipped or
// failed.
func (fs *Store) doWriteOp(oper *op) (done bool, err error) {
	keyStr := oper.key

	opRes := fs.opMap.Begin(keyStr)
	if opRes == nil { // nothing to do, a concurrent op succeeded
		return false, nil
	}

	err = fs.doOp(oper)

	// Finish it. If no error, it will signal other operations
	// waiting on this result to succeed. Otherwise, they will
	// retry.
	opRes.Finish(err == nil)
	return err == nil, err
}

func (fs *Store) doPut(key string, size uint64, val io.Reader) error {
	dir, path := fs.encode(key)
	if err := fs.makeDir(dir); err != nil {
		return err
	}

	tmp, err := fs.tempFile()
	if err != nil {
		return err
	}
	closed := false
	removed := false
	defer func() {
		if !closed {
			// silence errcheck
			_ = tmp.Close()
		}
		if !removed {
			// silence errcheck
			_ = os.Remove(tmp.Name())
		}
	}()

	n, err := io.Copy(tmp, val)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	if uint64(n) != size {
		log.Errorw("put object size mismatch", "key", key, "expected_size", size, "actual_size", n)
		return fmt.Errorf("put object size mismatch: got %d, expected %d", n, size)
	}
	if fs.sync {
		if err := syncFile(tmp); err != nil {
			return err
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	closed = true

	err = fs.rename(tmp.Name(), path)
	if err != nil {
		return err
	}
	removed = true

	if fs.sync {
		if err := syncDir(dir); err != nil {
			return err
		}
	}
	return nil
}

func (fs *Store) Get(ctx context.Context, key string, opts ...objectstore.GetOption) (objectstore.Object, error) {
	// Can't exist in datastore.
	if !keyIsValid(key) {
		return nil, objectstore.ErrNotExist
	}

	cfg := objectstore.NewGetConfig()
	cfg.ProcessOptions(opts)

	size, err := fs.getSize(key)
	if err != nil {
		return nil, fmt.Errorf("getting size: %w", err)
	}

	if !rangeSatisfiable(cfg.Range().Start, cfg.Range().End, uint64(size)) {
		return nil, ErrRangeNotSatisfiable
	}

	_, path := fs.encode(key)
	return FileObject{name: path, size: size, byteRange: cfg.Range()}, nil
}

func (fs *Store) getSize(key string) (size int64, err error) {
	// Can't exist in datastore.
	if !keyIsValid(key) {
		return -1, objectstore.ErrNotExist
	}

	_, path := fs.encode(key)
	switch s, err := os.Stat(path); {
	case err == nil:
		return int64(s.Size()), nil
	case os.IsNotExist(err):
		return -1, objectstore.ErrNotExist
	default:
		return -1, err
	}
}

// Delete removes a key/value from the Datastore. Please read
// the Put() explanation about the handling of concurrent write
// operations to the same key.
func (fs *Store) Delete(ctx context.Context, key string) error {
	// Can't exist in datastore.
	if !keyIsValid(key) {
		return nil
	}

	fs.shutdownLock.RLock()
	defer fs.shutdownLock.RUnlock()
	if fs.shutdown {
		return ErrClosed
	}

	_, err := fs.doWriteOp(&op{
		typ: opDelete,
		key: key,
		v:   nil,
	})
	return err
}

// This function always runs within an opLock for the given
// key, and not concurrently.
func (fs *Store) doDelete(key string) error {
	_, path := fs.encode(key)

	var err error
	for i := 0; i < RetryAttempts; i++ {
		err = os.Remove(path)
		if err == nil {
			break
		} else if os.IsNotExist(err) {
			return nil
		}
	}

	return err
}

func (fs *Store) tempFile() (*os.File, error) {
	file, err := tempFile(fs.tempPath, "temp-")
	return file, err
}

// Deactivate closes background maintenance threads, most write
// operations will fail but readonly operations will continue to
// function
func (fs *Store) deactivate() {
	fs.shutdownLock.Lock()
	defer fs.shutdownLock.Unlock()
	if fs.shutdown {
		return
	}
	fs.shutdown = true
}

func (fs *Store) Close() error {
	fs.deactivate()
	return nil
}
