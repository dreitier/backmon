package stat

// Common data structures for files. As S3 objects are also files, we are using our own filesystem abstraction.
import (
	"time"
	"os"
	"errors"
)

// DirectoryInfo has a list of containing files and subdirectories
type DirectoryInfo struct {
	Name    string
	SubDirs map[string]*DirectoryInfo
	Files   []*FileInfo
}

// FileInfo contains information about a file item
type FileInfo struct {
	// File name
	Name      	string
	// Absolute path to parent directory
	Parent      string
	Size      	int64
	// The timestamp when the file has been created for the first time, without counting any copies etc. 
	BornAt	  	time.Time
	// When has the last writing to this file occurred?
	// The difference between (ModifiedAt - BornAt) is the duration of the timeframe in seconds in which a file has been modified (e.g. how long a backup procedure has been taken)
	ModifiedAt	time.Time
	// When has the file been copied to long-time archival? In a local storage, this would be probably the same as ModifiedAt
	ArchivedAt	time.Time
}

func IsFilePathValid(path string) (bool, error) {
    _, err := os.Stat(path)
    if err == nil {
        return true, nil
    }
    if errors.Is(err, os.ErrNotExist) {
        return false, nil
    }
    return false, err
}