package provider

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	fs "github.com/dreitier/cloudmon/storage/fs"
)

type LocalClient struct {
	Directory string
	EnvName   string
}

func (c *LocalClient) GetFileNames(diskName string, maxDepth uint) (*fs.DirectoryInfo, error) {
	if diskName != c.Directory {
		return nil, errors.New(fmt.Sprintf("disk %#q does not exist", diskName))
	}

	return scanDir(diskName, "", "", maxDepth)
}

func scanDir(root string, path string, dir string, maxDepth uint) (*fs.DirectoryInfo, error) {
	path = filepath.Join(path, dir)
	fileInfos, err := ioutil.ReadDir(filepath.Join(root, path))
	if err != nil {
		//TODO: log error
		return nil, err
	}

	info := &fs.DirectoryInfo{
		Name:    dir,
		SubDirs: make(map[string]*fs.DirectoryInfo),
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
			file := &fs.FileInfo{
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

func (c *LocalClient) GetDiskNames() ([]string, error) {
	diskName := c.Directory
	diskNames := make([]string, 1, 1)
	diskNames[0] = diskName

	return diskNames, nil
}

func (c *LocalClient) Download(disk string, file *fs.FileInfo) (bytes io.ReadCloser, err error) {
	if disk != c.Directory {
		return nil, errors.New(fmt.Sprintf("disk %#q does not exist", disk))
	}
	fileName := filepath.Join(disk, file.Path, file.Name)

	bytes, err = os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for reading: %s", err)
	}

	return bytes, nil
}

func (c *LocalClient) Delete(disk string, file *fs.FileInfo) error {
	if disk != c.Directory {
		return fmt.Errorf("disk %#q does not exist", disk)
	}
	filePath := filepath.Join(disk, file.Path, file.Name)

	err := os.Remove(filePath)
	return err
}

func (c *LocalClient) findDisk(diskName *string) (*string, error) {
	names, err := c.GetDiskNames()

	if err != nil {
		return nil, fmt.Errorf("could not get available diskName names: %s", err)
	}

	if runtime.GOOS != "windows" && !strings.HasPrefix(*diskName, string(os.PathSeparator)) {
		*diskName = string(os.PathSeparator) + *diskName
	}

	diskFound := false

	for _, name := range names {
		if name == *diskName {
			diskFound = true
		}
	}

	if !diskFound {
		return nil, fmt.Errorf("unknown diskName %#q", *diskName)
	}

	return diskName, nil
}

func (c *LocalClient) getDiskName() string {
	trimmedString := strings.TrimLeft(c.Directory, "/")
	normalizedDiskName := strings.Replace(trimmedString, "/", "_", -1)
	return normalizedDiskName
}
