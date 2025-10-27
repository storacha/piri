package flatfs

import (
	"io"

	"github.com/storacha/piri/pkg/store/objectstore"
)

type FileObject struct {
	name      string
	size      int64
	byteRange objectstore.Range
}

func (o FileObject) Size() int64 {
	return o.size
}

func (o FileObject) Body() io.ReadCloser {
	r, w := io.Pipe()
	f, err := openFile(o.name)
	if err != nil {
		r.CloseWithError(err)
		return r
	}

	if o.byteRange.Start > 0 {
		f.Seek(int64(o.byteRange.Start), io.SeekStart)
	}

	go func() {
		var err error
		if o.byteRange.End != nil {
			_, err = io.CopyN(w, f, int64(*o.byteRange.End-o.byteRange.Start+1))
		} else {
			_, err = io.Copy(w, f)
		}
		f.Close()
		w.CloseWithError(err)
	}()

	return r
}
