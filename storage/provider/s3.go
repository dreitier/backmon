package provider

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/dreitier/cloudmon/config"
	storage "github.com/dreitier/cloudmon/storage/abstraction"
	log "github.com/sirupsen/logrus"
	"io"
	"strings"
)

type S3Client struct {
	Name           string
	AccessKey      string
	SecretKey      string
	Token          string
	Region         string
	Endpoint       string
	ForcePathStyle bool
	EnvName        string
	s3Client       *s3.S3
}

func getClient(c *S3Client) (*s3.S3, error) {
	if c.s3Client != nil {
		return c.s3Client, nil
	}

	cfg := aws.Config{
		Credentials: credentials.NewStaticCredentials(c.AccessKey, c.SecretKey, c.Token),
	}

	if c.ForcePathStyle {
		cfg.S3ForcePathStyle = aws.Bool(true)
	}

	if len(c.Region) > 0 {
		cfg.Region = aws.String(c.Region)
	} else {
		cfg.Region = aws.String("eu-central-1")
	}

	if len(c.Endpoint) > 0 {
		cfg.Endpoint = aws.String(c.Endpoint)
	}

	sess, err := session.NewSession(&cfg)

	if err != nil {
		return nil, fmt.Errorf("failed to build S3 client: %s", err)
	}

	c.s3Client = s3.New(sess)

	return c.s3Client, nil
}

func (c *S3Client) List(diskName *string) (files []*storage.File, err error) {
	svc, err := getClient(c)

	if err != nil {
		return nil, fmt.Errorf("could not acquire S3 client instance: %s", err)
	}

	// get items from the diskName
	result, err := svc.ListObjects(&s3.ListObjectsInput{Bucket: diskName})

	if err != nil {
		return nil, fmt.Errorf("failed to get objects in diskName: %s", err)
	}

	log.Infof("Retrieved %d items from disk %s", len(result.Contents), *diskName)

	// convert results to type File and append to list
	files = appendToFileList(files, result.Contents)

	// if the diskName held more than $maxKeys items, fetch them until we got them all
	for *result.IsTruncated {
		result, err = svc.ListObjects(&s3.ListObjectsInput{Bucket: diskName, Marker: result.NextMarker})
		if err != nil {
			return nil, fmt.Errorf("failed to get objects in diskName: %s", err)
		}

		log.Debugf("Retrieved %d items from disk %s", len(result.Contents), *diskName)

		files = appendToFileList(files, result.Contents)
	}

	return files, nil
}

func (c *S3Client) GetFileNames(diskName string, maxDepth uint) (*storage.DirectoryInfo, error) {
	svc, err := getClient(c)
	if err != nil {
		return nil, fmt.Errorf("could not acquire S3 client instance: %s", err)
	}

	// get items from the diskName
	result, err := svc.ListObjects(&s3.ListObjectsInput{Bucket: &diskName})
	if err != nil {
		return nil, fmt.Errorf("failed to get objects in disk %#q: %s", diskName, err)
	}
	log.Infof("Retrieved %d items from disk %#q", len(result.Contents), diskName)

	info := &storage.DirectoryInfo{
		Name:    diskName,
		SubDirs: make(map[string]*storage.DirectoryInfo),
	}

	appendFilesTo(info, result.Contents)

	// if the diskName held more than $maxKeys items, fetch them until we got them all
	for *result.IsTruncated {
		result, err = svc.ListObjects(&s3.ListObjectsInput{Bucket: &diskName, Marker: result.NextMarker})
		if err != nil {
			return nil, fmt.Errorf("failed to get objects in disk %#q: %s", diskName, err)
		}
		log.Infof("Retrieved %d items from disk %#q", len(result.Contents), diskName)

		appendFilesTo(info, result.Contents)
	}

	return info, nil
}

func appendFilesTo(root *storage.DirectoryInfo, objects []*s3.Object) {
	for _, obj := range objects {
		path := strings.Split(*obj.Key, "/")
		fileName := path[len(path)-1]
		path = path[0 : len(path)-1]
		currentDir := root
		for i := 0; i < len(path); i++ {
			next := currentDir.SubDirs[path[i]]
			if next == nil {
				next = &storage.DirectoryInfo{
					Name:    path[i],
					SubDirs: make(map[string]*storage.DirectoryInfo),
				}
				currentDir.SubDirs[path[i]] = next
			}
			currentDir = next
		}
		file := &storage.FileInfo{
			Name:      fileName,
			Path:      strings.Join(path, "/"),
			Timestamp: *obj.LastModified,
			Size:      *obj.Size,
		}
		currentDir.Files = append(currentDir.Files, file)
	}
}

func (c *S3Client) get(diskName *string, fileName *string) (file *s3.GetObjectOutput, err error) {
	svc, err := getClient(c)

	if err != nil {
		return nil, fmt.Errorf("could not acquire S3 client instance: %s", err)
	}
	getObjectInput := s3.GetObjectInput{Bucket: diskName, Key: fileName}
	out, err := svc.GetObject(&getObjectInput)

	if err != nil {
		return nil, err
	}

	return out, nil
}

func (c *S3Client) findAvailableDisks() ([]*s3.Bucket, error) {
	var r []*s3.Bucket

	svc, err := getClient(c)

	if err != nil {
		return nil, fmt.Errorf("Could not acquire S3 client instance: %s", err)
	}

	result, err := svc.ListBuckets(&s3.ListBucketsInput{})

	if err != nil {
		return nil, fmt.Errorf("Failed to list S3 disks: %s", err)
	}

	for _, disk := range result.Buckets {
		// don't try to list items in ignored disks
		diskName := *disk.Name
		if config.GetInstance().Global().IgnoreDisk(diskName) {
			log.Debugf("Not listing files in disk %s, because it's on the ignore list", diskName)
			continue
		}
		_, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{Bucket: aws.String(diskName)})

		if err != nil {
			log.Warnf("Unable to list items in S3 bucket %q, %v; won't use it as disk", disk, err)
			continue
		}

		r = append(r, disk)
	}

	return r, nil
}

func (c *S3Client) GetDiskNames() ([]string, error) {
	var diskNames []string

	disks, err := c.findAvailableDisks()

	if err != nil {
		return nil, fmt.Errorf("failed to get disk names: %v", err)
	}

	for _, disk := range disks {
		diskNames = append(diskNames, *disk.Name)
	}

	return diskNames, nil
}

func (c *S3Client) Download(disk string, file *storage.FileInfo) (bytes io.ReadCloser, err error) {
	fullName := file.Path + "/" + file.Name
	out, err := c.get(&disk, &fullName)

	if err != nil {
		return nil, fmt.Errorf("failed to download object %s from disk %s: %s", fullName, disk, err)
	}

	return out.Body, nil
}

func (c *S3Client) Delete(disk string, file *storage.FileInfo) error {
	//TODO: check out the s3 delete object documentation to make this work with versioned files
	svc, err := getClient(c)

	if err != nil {
		return fmt.Errorf("could not acquire S3 client instance: %s", err)
	}
	fullName := file.Path + "/" + file.Name
	delObjectInput := s3.DeleteObjectInput{Bucket: &disk, Key: &fullName}
	out, err := svc.DeleteObject(&delObjectInput)
	fmt.Sprint(out)

	if err != nil {
		return fmt.Errorf("failed to delete object %s from disk %s: %s", fullName, disk, err)
	}

	return nil
}

func appendToFileList(files []*storage.File, output []*s3.Object) []*storage.File {
	for _, item := range output {
		files = append(files, &storage.File{Name: item.Key, Timestamp: item.LastModified, Size: item.Size})
	}

	return files
}
