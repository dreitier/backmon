package storage

import (
	"io"

	"github.com/dreitier/backmon/config"
	fs "github.com/dreitier/backmon/storage/fs"
	"github.com/dreitier/backmon/storage/provider"
)

type Client interface {
	GetDiskNames() ([]string, error)

	GetFileNames(disk string, maxDepth uint64) (*fs.DirectoryInfo, error)

	Download(disk string, file *fs.FileInfo) (bytes io.ReadCloser, length int64, contentType string, err error)

	Delete(disk string, file *fs.FileInfo) error
}

func NewClient(config *config.ClientConfiguration) Client {
	if config.Directory == "" {
		return &provider.S3Client{
			EnvName:           config.EnvName,
			Region:            config.Region,
			AccessKey:         config.AccessKey,
			SecretKey:         config.SecretKey,
			AssumeRoleArn:     config.AssumeRoleArn,
			Endpoint:          config.Endpoint,
			TLSSkipVerify:     config.TLSSkipVerify,
			ForcePathStyle:    config.ForcePathStyle,
			Token:             config.Token,
			AutoDiscoverDisks: config.AutoDiscoverDisks,
			Disks:             config.Disks,
		}
	}

	// fall back to local client
	return &provider.LocalClient{
		EnvName:   config.EnvName,
		Directory: config.Directory,
	}
}
