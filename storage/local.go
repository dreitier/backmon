package storage

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type LocalClient struct {
	Directory string
	EnvName   string
}

func (c *LocalClient) GetFileNames(bucketName string, maxDepth uint) (*DirectoryInfo, error) {
	if bucketName != c.Directory {
		return nil, errors.New(fmt.Sprintf("bucket %#q does not exist", bucketName))
	}

	return scanDir(bucketName, "", "", maxDepth)
}

func scanDir(root string, path string, dir string, maxDepth uint) (*DirectoryInfo, error) {
	path = filepath.Join(path, dir)
	fileInfos, err := ioutil.ReadDir(filepath.Join(root, path))
	if err != nil {
		//TODO: log error
		return nil, err
	}

	info := &DirectoryInfo{
		Name:    dir,
		SubDirs: make(map[string]*DirectoryInfo),
	}
	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			if maxDepth < 1 {
				continue
			}
			subDir, subErr := scanDir(root, path, fileInfo.Name(), maxDepth-1)
			if subErr == nil {
				info.SubDirs[subDir.Name] = subDir
			}
		} else {
			file := &FileInfo{
				Name:      fileInfo.Name(),
				Path:      path,
				Timestamp: fileInfo.ModTime(),
				Size:      fileInfo.Size(),
			}

			info.Files = append(info.Files, file)
		}
	}
	return info, nil
}

func (c *LocalClient) GetBucketNames() ([]string, error) {
	bucketName := c.Directory
	bucketNames := make([]string, 1, 1)
	bucketNames[0] = bucketName

	return bucketNames, nil
}

func (c *LocalClient) Download(bucket string, file *FileInfo) (bytes io.ReadCloser, err error) {
	if bucket != c.Directory {
		return nil, errors.New(fmt.Sprintf("bucket %#q does not exist", bucket))
	}
	fileName := filepath.Join(bucket, file.Path, file.Name)

	bytes, err = os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for reading: %s", err)
	}

	return bytes, nil
}

func (c *LocalClient) Delete(bucket string, file *FileInfo) error {
	if bucket != c.Directory {
		return fmt.Errorf("bucket %#q does not exist", bucket)
	}
	filePath := filepath.Join(bucket, file.Path, file.Name)

	err := os.Remove(filePath)
	return err
}

func (c *LocalClient) findBucket(bucketName *string) (*string, error) {
	names, err := c.GetBucketNames()

	if err != nil {
		return nil, fmt.Errorf("could not get available bucketName names: %s", err)
	}

	if runtime.GOOS != "windows" && !strings.HasPrefix(*bucketName, string(os.PathSeparator)) {
		*bucketName = string(os.PathSeparator) + *bucketName
	}

	bucketFound := false

	for _, name := range names {
		if name == *bucketName {
			bucketFound = true
		}
	}

	if !bucketFound {
		return nil, fmt.Errorf("unknown bucketName %#q", *bucketName)
	}

	return bucketName, nil
}

func (c *LocalClient) getBucketName() string {
	trimmedString := strings.TrimLeft(c.Directory, "/")
	normalizedBucketName := strings.Replace(trimmedString, "/", "_", -1)
	return normalizedBucketName
}
