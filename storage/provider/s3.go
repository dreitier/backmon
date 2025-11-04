package provider

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	cfg "github.com/dreitier/backmon/config"
	fs "github.com/dreitier/backmon/storage/fs"
	dotstat "github.com/dreitier/backmon/storage/fs/dotstat"
	log "github.com/sirupsen/logrus"
)

type S3Client struct {
	Name              string
	AccessKey         string
	SecretKey         string
	Token             string
	Region            string
	Endpoint          string
	TLSSkipVerify     bool
	ForcePathStyle    bool
	EnvName           string
	s3Client          *s3.Client
	AutoDiscoverDisks bool
	Disks             *cfg.DisksConfiguration
}

func getClient(c *S3Client) (*s3.Client, error) {
	if c.s3Client != nil {
		return c.s3Client, nil
	}

	var awscfg = aws.Config{}

	if len(c.AccessKey) == 0 || len(c.SecretKey) == 0 {
		log.Debug("No access key or secret key provided, trying to use AWS credentials.")

		ctx := context.Background()
		awscfg, err := config.LoadDefaultConfig(ctx)

		if err != nil {
			log.Errorf("unable to load SDK config, %v", err)
			return nil, err
		}

		stsClient := sts.NewFromConfig(awscfg)
		callerIdentity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})

		if err != nil {
			log.Errorf("failed to get caller identity: %v", err)
			return nil, err
		}

		log.Debugf("Using Role ARN: %s\n", aws.ToString(callerIdentity.Arn))

	} else {
		awscfg.Credentials = credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, c.Token)
	}

	if len(c.Region) > 0 {
		awscfg.Region = c.Region
	} else {
		if len(awscfg.Region) == 0 {
			awscfg.Region = "eu-central-1"
		}
	}

	if c.TLSSkipVerify {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		httpClient := &http.Client{Transport: tr}
		awscfg.HTTPClient = httpClient
	}

	c.s3Client = s3.NewFromConfig(awscfg, func(o *s3.Options) {
		o.UsePathStyle = c.ForcePathStyle
		o.BaseEndpoint = aws.String(c.Endpoint)
	})

	return c.s3Client, nil
}

// GetFileNames TODO: do something smart with unused parameter maxDepth
func (c *S3Client) GetFileNames(diskName string, maxDepth uint64) (*fs.DirectoryInfo, error) {
	client, err := getClient(c)

	if err != nil {
		return nil, fmt.Errorf("could not acquire S3 client instance: %s", err)
	}

	var continuationToken *string

	bucketRoot := &fs.DirectoryInfo{
		Name:    diskName,
		SubDirs: make(map[string]*fs.DirectoryInfo),
	}

	dotStatFiles := make(map[string] /* path to regular file*/ string /* path to .stat file */)

	for {
		// get items from the diskName
		result, err := client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{Bucket: &diskName, ContinuationToken: continuationToken})

		if err != nil {
			return nil, fmt.Errorf("failed to get objects in disk %#q: %s", diskName, err)
		}

		log.Infof("Retrieved %d items from disk %#q", len(result.Contents), diskName)

		c.appendFilesTo(&diskName, bucketRoot, result.Contents, &dotStatFiles)

		if !*result.IsTruncated {
			break
		}

		continuationToken = result.NextContinuationToken
	}

	dotstat.ApplyDotStatValuesRecursively(dotStatFiles, bucketRoot)
	c.cleanupTemporaryFiles(&dotStatFiles)

	return bucketRoot, nil
}

// Clean up files which have been temporarily downloaded for .stat introspection
func (c *S3Client) cleanupTemporaryFiles(dotStatFiles *map[string] /* path to regular file*/ string /* path to .stat file */) {
	for _, pathToDotStatFile := range *dotStatFiles {
		log.Debugf("Removing temporary file %s", pathToDotStatFile)
		err := os.Remove(pathToDotStatFile)

		if err != nil {
			log.Errorf("Unable to remove temporary file %s: %s", pathToDotStatFile, err)
		}
	}
}

func (c *S3Client) appendFilesTo(diskName *string, root *fs.DirectoryInfo, objects []types.Object, dotStatFiles *map[string] /* path to regular file*/ string /* path to .stat file */) {
	for _, obj := range objects {
		pathSegments := strings.Split(*obj.Key, "/")
		fileName := pathSegments[len(pathSegments)-1]
		pathSegments = pathSegments[0 : len(pathSegments)-1]
		currentDir := root

		for i := 0; i < len(pathSegments); i++ {
			next := currentDir.SubDirs[pathSegments[i]]

			if next == nil {
				next = &fs.DirectoryInfo{
					Name:    pathSegments[i],
					SubDirs: make(map[string]*fs.DirectoryInfo),
				}

				currentDir.SubDirs[pathSegments[i]] = next
			}

			currentDir = next
		}

		parentPath := strings.Join(pathSegments, "/")

		// if object is a .stat file, it is downloaded for later introspection
		if dotstat.IsStatFile(fileName) {
			s3PathToStatFile := parentPath + "/" + fileName
			s3PathToNonStatFile := dotstat.RemoveDotStatSuffix(s3PathToStatFile)

			tempFile, err := os.CreateTemp(os.TempDir(), "backmon_"+strings.ReplaceAll(strings.ReplaceAll(parentPath, "/", "_"), "\\", "_"))

			if err != nil {
				log.Errorf("Unable to create temporary file for .stat: %s", err)
				continue
			}

			localAbsolutePath := tempFile.Name()

			// .stat files are registered for later examination
			log.Debugf("Found .stat file %s for %s; downloading .stat file and writing content to local path %s", s3PathToStatFile, s3PathToNonStatFile, localAbsolutePath)
			s3OutObject, _ := c.get(diskName, &s3PathToStatFile)
			byteStreamContent, _ := io.ReadAll(s3OutObject.Body)

			_, err = tempFile.Write(byteStreamContent)
			if err != nil {
				log.Errorf("failed to write to file: %s", err)
				return
			}
			(*dotStatFiles)[s3PathToNonStatFile] = localAbsolutePath

			continue
		}

		file := &fs.FileInfo{
			Name:       fileName,
			Parent:     parentPath,
			BornAt:     *obj.LastModified,
			ModifiedAt: *obj.LastModified,
			ArchivedAt: *obj.LastModified,
			Size:       *obj.Size,
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
	out, err := svc.GetObject(context.Background(), &getObjectInput)

	if err != nil {
		return nil, err
	}

	return out, nil
}

func (c *S3Client) GetDiskNames() ([]string, error) {
	svc, err := getClient(c)

	if err != nil {
		return nil, fmt.Errorf("could not acquire S3 client instance: %s", err)
	}

	if c.AutoDiscoverDisks {
		return c.findAvailableDisksByAutoDiscovery(svc)
	}

	return c.findAvailableDisksByInclusion(svc)
}

// Find available disks by iterating over each available bucket. That assumes that the AWS user has the IAM permission `ListAllMyBuckets`
// @see #9
func (c *S3Client) findAvailableDisksByAutoDiscovery(svc *s3.Client) ([]string, error) {
	var r []string

	log.Info("Auto-discovering disks based upon available S3 buckets...")
	result, err := svc.ListBuckets(context.Background(), &s3.ListBucketsInput{})

	if err != nil {
		return nil, fmt.Errorf("failed to list S3 disks by auto discovery: %s", err)
	}

	for _, bucketAsDisk := range result.Buckets {
		if c.hasAccessToBucket(svc, bucketAsDisk.Name) {
			r = append(r, *bucketAsDisk.Name)
		}
	}

	return r, nil
}

// Find available disks by iterating over disks.include configuration parameter
func (c *S3Client) findAvailableDisksByInclusion(svc *s3.Client) ([]string, error) {
	var r []string

	log.Info("Finding disks based upon disks.include configuration parameter...")

	for keyAsBucketName := range c.Disks.GetIncludedDisks() {
		if c.hasAccessToBucket(svc, &keyAsBucketName) {
			r = append(r, keyAsBucketName)
		}
	}

	return r, nil
}

// Check if objects from the bucket can be retrieved. It is basically a test for the IAM permission for GetObject
func (c *S3Client) hasAccessToBucket(svc *s3.Client, bucketName *string) bool {
	// don't try to list items in ignored disks
	if !c.Disks.IsDiskIncluded(*bucketName) {
		return false
	}

	_, err := svc.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{Bucket: aws.String(*bucketName)})

	if err != nil {
		log.Warnf("Unable to list items in S3 bucket %q, %v; won't use it as disk", *bucketName, err)
		return false
	}

	return true
}

func (c *S3Client) Download(disk string, file *fs.FileInfo) (bytes io.ReadCloser, err error) {
	fullName := file.Parent + "/" + file.Name
	out, err := c.get(&disk, &fullName)

	if err != nil {
		return nil, fmt.Errorf("failed to download object %s from disk %s: %s", fullName, disk, err)
	}

	return out.Body, nil
}

func (c *S3Client) Delete(disk string, file *fs.FileInfo) error {
	//TODO: check out the s3 delete object documentation to make this work with versioned files
	svc, err := getClient(c)

	if err != nil {
		return fmt.Errorf("could not acquire S3 client instance: %s", err)
	}
	fullName := file.Parent + "/" + file.Name
	delObjectInput := s3.DeleteObjectInput{Bucket: &disk, Key: &fullName}
	out, err := svc.DeleteObject(context.Background(), &delObjectInput)
	_ = fmt.Sprint(out)

	if err != nil {
		return fmt.Errorf("failed to delete object %s from disk %s: %s", fullName, disk, err)
	}

	return nil
}
