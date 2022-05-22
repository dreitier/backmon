package storage

import (
	"github.com/dreitier/cloudmon/config"
	"io"
)

type Client interface {
	GetBucketNames() ([]string, error)
	GetFileNames(bucket string, maxDepth uint) (*DirectoryInfo, error)
	Download(bucket string, file *FileInfo) (bytes io.ReadCloser, err error)
	Delete(bucket string, file *FileInfo) error
}

func NewClient(config *config.Client) Client {
	if config.Directory == "" {
		return &S3Client{
			EnvName:        config.EnvName,
			Region:         config.Region,
			AccessKey:      config.AccessKey,
			SecretKey:      config.SecretKey,
			Endpoint:       config.Endpoint,
			ForcePathStyle: config.ForcePathStyle,
			Token:          config.Token,
		}
	} else {
		return &LocalClient{
			EnvName:   config.EnvName,
			Directory: config.Directory,
		}
	}
}
