package fs

// Common data structures for files. As S3 objects are also files, we are using our own filesystem abstraction.
import (
	"time"
)

// DirectoryInfo has a list of containing files and subdirectories
type DirectoryInfo struct {
	Name    string
	SubDirs map[string]*DirectoryInfo
	Files   []*FileInfo
}

// FileInfo contains information about a file item
type FileInfo struct {
	Name      string
	Path      string
	Size      int64
	Timestamp time.Time
}