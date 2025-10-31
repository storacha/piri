package flatfs_test

import (
	"bytes"
	"context"
	"encoding/base32"
	"errors"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/storacha/piri/pkg/store/objectstore"
	flatfs "github.com/storacha/piri/pkg/store/objectstore/flatfs"
)

var bg = context.Background()

func checkTemp(t *testing.T, dir string) {
	tempDir, err := os.Open(filepath.Join(dir, ".temp"))
	if err != nil {
		t.Errorf("failed to open temp dir: %s", err)
		return
	}

	names, err := tempDir.Readdirnames(-1)
	tempDir.Close()

	if err != nil {
		t.Errorf("failed to read temp dir: %s", err)
		return
	}

	for _, name := range names {
		t.Errorf("found leftover temporary file: %s", name)
	}
}

func tempdir(t testing.TB) (path string, cleanup func()) {
	path, err := os.MkdirTemp("", "test-datastore-flatfs-")
	if err != nil {
		t.Fatalf("cannot create temp directory: %v", err)
	}

	cleanup = func() {
		if err := os.RemoveAll(path); err != nil {
			t.Errorf("tempdir cleanup failed: %v", err)
		}
	}
	return path, cleanup
}

func tryAllShardFuncs(t *testing.T, testFunc func(mkShardFunc, *testing.T)) {
	t.Run("prefix", func(t *testing.T) { testFunc(flatfs.Prefix, t) })
	t.Run("suffix", func(t *testing.T) { testFunc(flatfs.Suffix, t) })
	t.Run("next-to-last", func(t *testing.T) { testFunc(flatfs.NextToLast, t) })
}

type mkShardFunc func(int) *flatfs.ShardIdV1

func testPut(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()
	defer checkTemp(t, temp)

	fs, err := flatfs.New(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	input := "foobar"
	err = fs.Put(bg, "quux", uint64(len(input)), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	input = "nonono"
	err = fs.Put(bg, "FOO", uint64(len(input)), strings.NewReader(input))
	if err == nil {
		t.Fatalf("did not expect to put a uppercase key")
	}
}

func TestPut(t *testing.T) { tryAllShardFuncs(t, testPut) }

func testGet(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()
	defer checkTemp(t, temp)

	fs, err := flatfs.New(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	const input = "foobar"
	err = fs.Put(bg, "quux", uint64(len(input)), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	obj, err := fs.Get(bg, "quux")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	g, err := io.ReadAll(obj.Body())
	if err != nil {
		t.Fatalf("Read all failed: %v", err)
	}
	if string(g) != input {
		t.Fatalf("Get gave wrong content: %q != %q", string(g), input)
	}

	_, err = fs.Get(bg, "FOO/BAR")
	if err != objectstore.ErrNotExist {
		t.Fatalf("expected ErrNotExist, got %s", err)
	}
}

func TestGet(t *testing.T) { tryAllShardFuncs(t, testGet) }

func testGetRange(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()
	defer checkTemp(t, temp)

	fs, err := flatfs.New(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	const input = "foobar"
	err = fs.Put(bg, "quux", uint64(len(input)), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	start := uint64(3)
	end := uint64(5)
	obj, err := fs.Get(bg, "quux", objectstore.WithRange(objectstore.Range{Start: start, End: &end}))
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	g, err := io.ReadAll(obj.Body())
	if err != nil {
		t.Fatalf("Read all failed: %v", err)
	}
	if obj.Size() != int64(len(input)) {
		t.Fatalf("Get gave wrong size: %d != %d", obj.Size(), len(input))
	}
	if string(g) != input[start:end+1] {
		t.Fatalf("Get gave wrong content: %q != %q", string(g), input)
	}
}

func TestGetRange(t *testing.T) { tryAllShardFuncs(t, testGetRange) }

func testPutOverwrite(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()
	defer checkTemp(t, temp)

	fs, err := flatfs.New(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	const (
		loser  = "foobar"
		winner = "xyzzy"
	)
	err = fs.Put(bg, "quux", uint64(len(loser)), strings.NewReader(loser))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	err = fs.Put(bg, "quux", uint64(len(winner)), strings.NewReader(winner))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	obj, err := fs.Get(bg, "quux")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	g, err := io.ReadAll(obj.Body())
	if err != nil {
		t.Fatalf("Read all failed: %v", err)
	}
	if string(g) != winner {
		t.Fatalf("Get gave wrong content: %q != %q", g, winner)
	}
}

func TestPutOverwrite(t *testing.T) { tryAllShardFuncs(t, testPutOverwrite) }

func testGetNotFoundError(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()
	defer checkTemp(t, temp)

	fs, err := flatfs.New(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	_, err = fs.Get(bg, "quux")
	if !errors.Is(err, objectstore.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got: %v\n", err)
	}
}

func TestGetNotFoundError(t *testing.T) { tryAllShardFuncs(t, testGetNotFoundError) }

type params struct {
	shard *flatfs.ShardIdV1
	dir   string
	key   string
}

func testStorage(p *params, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()
	defer checkTemp(t, temp)

	target := p.dir + string(os.PathSeparator) + p.key + ".data"
	fs, err := flatfs.New(temp, p.shard, false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	value := []byte("foobar")
	err = fs.Put(bg, p.key, uint64(len(value)), bytes.NewReader(value))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	fs.Close()
	seen := false
	haveREADME := false
	walk := func(absPath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		path, err := filepath.Rel(temp, absPath)
		if err != nil {
			return err
		}
		switch path {
		case ".", "..", "SHARDING", ".temp":
			// ignore
		case "_README":
			_, err := os.ReadFile(absPath)
			if err != nil {
				t.Error("could not read _README file")
			}
			haveREADME = true
		case p.dir:
			if !fi.IsDir() {
				t.Errorf("directory is not a file? %v", fi.Mode())
			}
			// we know it's there if we see the file, nothing more to
			// do here
		case target:
			seen = true
			if !fi.Mode().IsRegular() {
				t.Errorf("expected a regular file, mode: %04o", fi.Mode())
			}
		default:
			t.Errorf("saw unexpected directory entry: %q %v", path, fi.Mode())
		}
		return nil
	}
	if err := filepath.Walk(temp, walk); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if !seen {
		t.Error("did not see the data file")
	}
	if fs.ShardStr() == flatfs.NEXT_TO_LAST2_DEF_SHARD.String() && !haveREADME {
		t.Error("expected _README file")
	} else if fs.ShardStr() != flatfs.NEXT_TO_LAST2_DEF_SHARD.String() && haveREADME {
		t.Error("did not expect _README file")
	}
}

func TestStorage(t *testing.T) {
	t.Run("prefix", func(t *testing.T) {
		testStorage(&params{
			shard: flatfs.Prefix(2),
			dir:   "qu",
			key:   "quux",
		}, t)
	})
	t.Run("suffix", func(t *testing.T) {
		testStorage(&params{
			shard: flatfs.Suffix(2),
			dir:   "ux",
			key:   "quux",
		}, t)
	})
	t.Run("next-to-last", func(t *testing.T) {
		testStorage(&params{
			shard: flatfs.NextToLast(2),
			dir:   "uu",
			key:   "quux",
		}, t)
	})
}

func testDeleteNotFound(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()
	defer checkTemp(t, temp)

	fs, err := flatfs.New(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	err = fs.Delete(bg, "quux")
	if err != nil {
		t.Fatalf("expected nil, got: %v\n", err)
	}
}

func TestDeleteNotFound(t *testing.T) { tryAllShardFuncs(t, testDeleteNotFound) }

func testDeleteFound(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()
	defer checkTemp(t, temp)

	fs, err := flatfs.New(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	value := []byte("foobar")
	err = fs.Put(bg, "quux", uint64(len(value)), bytes.NewReader(value))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	err = fs.Delete(bg, "quux")
	if err != nil {
		t.Fatalf("Delete fail: %v\n", err)
	}

	// check that it's gone
	_, err = fs.Get(bg, "quux")
	if !errors.Is(err, objectstore.ErrNotExist) {
		t.Fatalf("expected Get after Delete to give ErrNotExist, got: %v\n", err)
	}
}

func TestDeleteFound(t *testing.T) { tryAllShardFuncs(t, testDeleteFound) }

func testClose(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()
	defer checkTemp(t, temp)

	fs, err := flatfs.New(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}

	value := []byte("foobar")
	err = fs.Put(bg, "quux", uint64(len(value)), bytes.NewReader(value))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	fs.Close()

	err = fs.Put(bg, "qaax", uint64(len(value)), bytes.NewReader(value))
	if err == nil {
		t.Fatal("expected put on closed datastore to fail")
	}
}

func TestClose(t *testing.T) { tryAllShardFuncs(t, testClose) }

func TestSHARDINGFile(t *testing.T) {
	tempdir, cleanup := tempdir(t)
	defer cleanup()

	fun := flatfs.NEXT_TO_LAST2_DEF_SHARD

	fs, err := flatfs.New(tempdir, fun, false)
	if err != nil {
		t.Fatalf("Open fail: %v\n", err)
	}
	if fs.ShardStr() != flatfs.NEXT_TO_LAST2_DEF_SHARD.String() {
		t.Fatalf("Expected '%s' for shard function got '%s'", flatfs.NEXT_TO_LAST2_DEF_SHARD.String(), fs.ShardStr())
	}
	fs.Close()

	fs, err = flatfs.New(tempdir, fun, false)
	if err != nil {
		t.Fatalf("Could not reopen repo: %v\n", err)
	}
	fs.Close()

	fs, err = flatfs.New(tempdir, flatfs.Prefix(5), false)
	if err == nil {
		fs.Close()
		t.Fatalf("Was able to open repo with incompatible sharding function")
	}
}

func TestInvalidPrefix(t *testing.T) {
	_, err := flatfs.ParseShardFunc("/bad/prefix/v1/next-to-last/2")
	if err == nil {
		t.Fatalf("Expected an error while parsing a shard identifier with a bad prefix")
	}
}

func TestNonDatastoreDir(t *testing.T) {
	tempdir, cleanup := tempdir(t)
	defer cleanup()

	err := os.WriteFile(filepath.Join(tempdir, "afile"), []byte("Some Content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = flatfs.New(tempdir, flatfs.NextToLast(2), false)
	if err == nil {
		t.Fatalf("Expected an error when creating a datastore in a non-empty directory")
	}
}

func BenchmarkConsecutivePut(b *testing.B) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var blocks [][]byte
	var keys []string
	for i := 0; i < b.N; i++ {
		blk := make([]byte, 256*1024)
		r.Read(blk)
		blocks = append(blocks, blk)

		key := base32.StdEncoding.EncodeToString(blk[:8])
		keys = append(keys, key)
	}
	temp, cleanup := tempdir(b)
	defer cleanup()

	fs, err := flatfs.New(temp, flatfs.Prefix(2), false)
	if err != nil {
		b.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := fs.Put(bg, keys[i], uint64(len(blocks[i])), bytes.NewReader(blocks[i]))
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer() // avoid counting cleanup
}
