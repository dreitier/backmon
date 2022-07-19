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