package venus

import (
	"github.com/eikenb/pipeat"
	"io"
	"os"
)

// File defines an interface representing a SFTPGo file
type File interface {
	io.Reader
	io.Writer
	io.Closer
	io.ReaderAt
	io.WriterAt
	io.Seeker
	Stat() (os.FileInfo, error)
	Name() string
	Truncate(size int64) error
}

type fs interface {
	// Open 打开一个文件
	Open(name string, offset int64) (File, *pipeat.PipeReaderAt, func(), error)
	//
}
