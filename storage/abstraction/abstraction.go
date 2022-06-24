package abstraction

import (
	"time"
)

type DirectoryInfo struct {
	Name    string
	SubDirs map[string]*DirectoryInfo
	Files   []*FileInfo
}

type FileInfo struct {
	Name      string
	Path      string
	Size      int64
	Timestamp time.Time
}

type File struct {
	Name      *string
	Timestamp *time.Time
	Size      *int64
}