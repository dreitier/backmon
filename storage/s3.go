package storage

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
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

	if c.s3Client == nil {

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

	}

	return c.s3Client, nil
}

func (c *S3Client) List(bucketName *string) (files []*File, err error) {
	svc, err := getClient(c)

	if err != nil {
		return nil, fmt.Errorf("could not acquire S3 client instance: %s", err)
	}

	// get items from the bucketName
	result, err := svc.ListObjects(&s3.ListObjectsInput{Bucket: bucketName})

	if err != nil {
		return nil, fmt.Errorf("failed to get objects in bucketName: %s", err)
	}

	log.Infof("Retrieved %d items from bucket %s", len(result.Contents), *bucketName)

	// convert results to type File and append to list
	files = appendToFileList(files, result.Contents)

	// if the bucketName held more than $maxKeys items, fetch them until we got them all
	for *result.IsTruncated {
		result, err = svc.ListObjects(&s3.ListObjectsInput{Bucket: bucketName, Marker: result.NextMarker})
		if err != nil {
			return nil, fmt.Errorf("failed to get objects in bucketName: %s", err)
		}

		log.Debugf("Retrieved %d items from bucket %s", len(result.Contents), *bucketName)

		files = appendToFileList(files, result.Contents)
	}

	return files, nil
}

func (c *S3Client) GetFileNames(bucketName string, maxDepth uint) (*DirectoryInfo, error) {
	svc, err := getClient(c)
	if err != nil {
		return nil, fmt.Errorf("could not acquire S3 client instance: %s", err)
	}

	// get items from the bucketName
	result, err := svc.ListObjects(&s3.ListObjectsInput{Bucket: &bucketName})
	if err != nil {
		return nil, fmt.Errorf("failed to get objects in bucket %#q: %s", bucketName, err)
	}
	log.Infof("Retrieved %d items from bucket %#q", len(result.Contents), bucketName)

	info := &DirectoryInfo{
		Name:    bucketName,
		SubDirs: make(map[string]*DirectoryInfo),
	}

	appendFilesTo(info, result.Contents)

	// if the bucketName held more than $maxKeys items, fetch them until we got them all
	for *result.IsTruncated {
		result, err = svc.ListObjects(&s3.ListObjectsInput{Bucket: &bucketName, Marker: result.NextMarker})
		if err != nil {
			return nil, fmt.Errorf("failed to get objects in bucket %#q: %s", bucketName, err)
		}
		log.Infof("Retrieved %d items from bucket %#q", len(result.Contents), bucketName)

		appendFilesTo(info, result.Contents)
	}

	return info, nil
}

func appendFilesTo(root *DirectoryInfo, objects []*s3.Object) {
	for _, obj := range objects {
		path := strings.Split(*obj.Key, "/")
		fileName := path[len(path)-1]
		path = path[0 : len(path)-1]
		currentDir := root
		for i := 0; i < len(path); i++ {
			next := currentDir.SubDirs[path[i]]
			if next == nil {
				next = &DirectoryInfo{
					Name:    path[i],
					SubDirs: make(map[string]*DirectoryInfo),
				}
				currentDir.SubDirs[path[i]] = next
			}
			currentDir = next
		}
		file := &FileInfo{
			Name:      fileName,
			Path:      strings.Join(path, "/"),
			Timestamp: *obj.LastModified,
			Size:      *obj.Size,
		}
		currentDir.Files = append(currentDir.Files, file)
	}
}

func (c *S3Client) get(bucketName *string, fileName *string) (file *s3.GetObjectOutput, err error) {
	svc, err := getClient(c)

	if err != nil {
		return nil, fmt.Errorf("could not acquire S3 client instance: %s", err)
	}
	getObjectInput := s3.GetObjectInput{Bucket: bucketName, Key: fileName}
	out, err := svc.GetObject(&getObjectInput)

	if err != nil {
		return nil, err
	}

	return out, nil
}

func (c *S3Client) findAvailableBuckets() ([]*s3.Bucket, error) {

	svc, err := getClient(c)

	if err != nil {
		return nil, fmt.Errorf("could not acquire S3 client instance: %s", err)
	}

	result, err := svc.ListBuckets(&s3.ListBucketsInput{})

	if err != nil {
		return nil, fmt.Errorf("failed to list S3 buckets: %s", err)
	}

	return result.Buckets, nil
}

func (c *S3Client) GetBucketNames() ([]string, error) {
	var bucketNames []string

	buckets, err := c.findAvailableBuckets()

	if err != nil {
		return nil, fmt.Errorf("failed to get bucket names: %v", err)
	}

	for _, bucket := range buckets {
		bucketNames = append(bucketNames, *bucket.Name)
	}

	return bucketNames, nil
}

func (c *S3Client) Download(bucket string, file *FileInfo) (bytes io.ReadCloser, err error) {
	fullName := file.Path + "/" + file.Name
	out, err := c.get(&bucket, &fullName)

	if err != nil {
		return nil, fmt.Errorf("failed to download object %s from bucket %s: %s", fullName, bucket, err)
	}

	return out.Body, nil
}

func (c *S3Client) Delete(bucket string, file *FileInfo) error {
	//TODO: check out the s3 delete object documentation to make this work with versioned files
	svc, err := getClient(c)

	if err != nil {
		return fmt.Errorf("could not acquire S3 client instance: %s", err)
	}
	fullName := file.Path + "/" + file.Name
	delObjectInput := s3.DeleteObjectInput{Bucket: &bucket, Key: &fullName}
	out, err := svc.DeleteObject(&delObjectInput)
	fmt.Sprint(out)

	if err != nil {
		return fmt.Errorf("failed to delete object %s from bucket %s: %s", fullName, bucket, err)
	}

	return nil
}

func appendToFileList(files []*File, output []*s3.Object) []*File {
	for _, item := range output {
		files = append(files, &File{Name: item.Key, Timestamp: item.LastModified, Size: item.Size})
	}

	return files
}
