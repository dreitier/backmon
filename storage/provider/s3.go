package provider

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/dreitier/backmon/config"
	fs "github.com/dreitier/backmon/storage/fs"
	dotstat "github.com/dreitier/backmon/storage/fs/dotstat"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

type S3Client struct {
	Name              string
	AccessKey         string
	SecretKey         string
	Token             string
	Region            string
	Endpoint          string
	ForcePathStyle    bool
	EnvName           string
	s3Client          *s3.S3
	AutoDiscoverDisks bool
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

func (c *S3Client) GetFileNames(diskName string, maxDepth uint) (*fs.DirectoryInfo, error) {
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

	bucketRoot := &fs.DirectoryInfo{
		Name:    diskName,
		SubDirs: make(map[string]*fs.DirectoryInfo),
	}

	dotStatFiles := make(map[string] /* path to regular file*/ string /* path to .stat file */)

	c.appendFilesTo(&diskName, bucketRoot, result.Contents, &dotStatFiles)

	// if the diskName held more than $maxKeys items, fetch them until we got them all
	for *result.IsTruncated {
		result, err = svc.ListObjects(&s3.ListObjectsInput{Bucket: &diskName, Marker: result.NextMarker})

		if err != nil {
			return nil, fmt.Errorf("failed to get objects in disk %#q: %s", diskName, err)
		}

		log.Infof("Retrieved %d items from disk %#q", len(result.Contents), diskName)

		c.appendFilesTo(&diskName, bucketRoot, result.Contents, &dotStatFiles)
	}

	dotstat.ApplyDotStatValuesRecursively(dotStatFiles, bucketRoot)
	c.cleanupTemporaryFiles(&dotStatFiles)

	return bucketRoot, nil
}

// Clean up files which have been temporary downloaded for .stat introspection
func (c *S3Client) cleanupTemporaryFiles(dotStatFiles *map[string] /* path to regular file*/ string /* path to .stat file */) {
	for _, pathToDotStatFile := range *dotStatFiles {
		log.Debugf("Removing temporary file %s", pathToDotStatFile)
		err := os.Remove(pathToDotStatFile)

		if err != nil {
			log.Errorf("Unable to remove temporary file %s: %s", pathToDotStatFile, err)
		}
	}
}

func (c *S3Client) appendFilesTo(diskName *string, root *fs.DirectoryInfo, objects []*s3.Object, dotStatFiles *map[string] /* path to regular file*/ string /* path to .stat file */) {
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

			// TODO make temp directory configurable
			tempFile, err := ioutil.TempFile(os.TempDir(), "backmon_"+strings.ReplaceAll(strings.ReplaceAll(parentPath, "/", "_"), "\\", "_"))

			if err != nil {
				log.Errorf("Unable to create temporary file for .stat: %s", err)
				continue
			}

			localAbsolutePath := tempFile.Name()

			// .stat files are registered for later examination
			log.Debugf("Found .stat file %s for %s; downloading .stat file and writing content to local path %s", s3PathToStatFile, s3PathToNonStatFile, localAbsolutePath)
			s3OutObject, _ := c.get(diskName, &s3PathToStatFile)
			byteStreamContent, _ := ioutil.ReadAll(s3OutObject.Body)

			tempFile.Write(byteStreamContent)
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
	out, err := svc.GetObject(&getObjectInput)

	if err != nil {
		return nil, err
	}

	return out, nil
}

func (c *S3Client) GetDiskNames() ([]string, error) {
	svc, err := getClient(c)

	if err != nil {
		return nil, fmt.Errorf("Could not acquire S3 client instance: %s", err)
	}

	if c.AutoDiscoverDisks {
		return c.findAvailableDisksByAutoDiscovery(svc)
	}

	return c.findAvailableDisksByInclusion(svc)
}

// Find available disks by iterating over each available bucket. That assumes that the AWS user has the IAM permission `ListAllMyBuckets`
// @see #9
func (c *S3Client) findAvailableDisksByAutoDiscovery(svc *s3.S3) ([]string, error) {
	var r []string

	log.Info("Auto-discovering disks based upon available S3 buckets...")
	result, err := svc.ListBuckets(&s3.ListBucketsInput{})

	if err != nil {
		return nil, fmt.Errorf("Failed to list S3 disks by auto discovery: %s", err)
	}

	for _, bucketAsDisk := range result.Buckets {
		if c.hasAccessToBucket(svc, bucketAsDisk.Name) {
			r = append(r, *bucketAsDisk.Name)
		}
	}

	return r, nil
}

// Find available disks by iterating over disks.include configuration parameter
func (c *S3Client) findAvailableDisksByInclusion(svc *s3.S3) ([]string, error) {
	var r []string

	log.Info("Finding disks based upon disks.include configuration parameter...")

	for keyAsBucketName, _ := range config.GetInstance().Disks().GetIncludedDisks() {
		if c.hasAccessToBucket(svc, &keyAsBucketName) {
			r = append(r, keyAsBucketName)
		}
	}

	return r, nil
}

// Check if objects from the bucket can be retrieved. It is basically a test for the IAM permission for GetObject
func (c *S3Client) hasAccessToBucket(svc *s3.S3, bucketName *string) bool {
	// don't try to list items in ignored disks
	if !config.GetInstance().Disks().IsDiskIncluded(*bucketName) {
		return false
	}

	_, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{Bucket: aws.String(*bucketName)})

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
	out, err := svc.DeleteObject(&delObjectInput)
	fmt.Sprint(out)

	if err != nil {
		return fmt.Errorf("failed to delete object %s from disk %s: %s", fullName, disk, err)
	}

	return nil
}
