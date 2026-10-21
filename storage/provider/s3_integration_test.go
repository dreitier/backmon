package provider

// Integration tests for the S3 client against an S3-compatible server.
// They are skipped unless BACKMON_TEST_S3_ENDPOINT is set; run them locally with `make test-s3`
// (starts S3Mock in Docker), CI runs them against an S3Mock service container.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cfg "github.com/dreitier/backmon/config"
	fs "github.com/dreitier/backmon/storage/fs"
)

const s3EndpointEnv = "BACKMON_TEST_S3_ENDPOINT"

// newTestS3Client returns a client for the test server; disksYaml is the `disks:` section of the environment
func newTestS3Client(t *testing.T, disksYaml string) *S3Client {
	t.Helper()

	endpoint := os.Getenv(s3EndpointEnv)
	if endpoint == "" {
		t.Skipf("%s is not set, skipping S3 integration test", s3EndpointEnv)
	}

	if disksYaml == "" {
		disksYaml = "{}"
	}
	raw, err := cfg.Parse(strings.NewReader(disksYaml))
	require.NoError(t, err)

	return &S3Client{
		AccessKey:         "test",
		SecretKey:         "test",
		Region:            "us-east-1",
		Endpoint:          endpoint,
		ForcePathStyle:    true,
		EnvName:           "test",
		AutoDiscoverDisks: true,
		Disks:             cfg.ParseDisksSection(raw),
	}
}

// createBucket creates a bucket with a unique name and the given objects (key -> content);
// the bucket is removed when the test has finished
func createBucket(t *testing.T, c *S3Client, prefix string, objects map[string]string) string {
	t.Helper()

	client, err := getClient(c)
	require.NoError(t, err)

	suffix := make([]byte, 4)
	_, err = rand.Read(suffix)
	require.NoError(t, err)
	bucket := prefix + "-" + hex.EncodeToString(suffix)

	_, err = client.CreateBucket(context.Background(), &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	require.NoError(t, err)
	t.Cleanup(func() { removeBucket(t, client, bucket) })

	putObjects(t, client, bucket, objects)

	return bucket
}

func putObjects(t *testing.T, client *s3.Client, bucket string, objects map[string]string) {
	t.Helper()

	var wg sync.WaitGroup
	errs := make(chan error, len(objects))
	// limit the number of parallel uploads
	sem := make(chan struct{}, 16)

	for key, content := range objects {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			_, err := client.PutObject(context.Background(), &s3.PutObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
				Body:   strings.NewReader(content),
			})
			if err != nil {
				errs <- fmt.Errorf("put %s: %w", key, err)
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
}

func removeBucket(t *testing.T, client *s3.Client, bucket string) {
	ctx := context.Background()
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			t.Logf("cleanup: unable to list objects in %s: %s", bucket, err)
			return
		}
		for _, obj := range page.Contents {
			_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: obj.Key})
		}
	}

	if _, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Logf("cleanup: unable to delete bucket %s: %s", bucket, err)
	}
}

func objectExists(t *testing.T, c *S3Client, bucket string, key string) bool {
	t.Helper()

	client, err := getClient(c)
	require.NoError(t, err)

	_, err = client.HeadObject(context.Background(), &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err == nil {
		return true
	}

	var notFound *types.NotFound
	require.True(t, errors.As(err, &notFound), "unexpected error for HEAD %s: %s", key, err)

	return false
}

// getFileNames calls GetFileNames and turns a panic into a test failure, so that one broken case
// doesn't abort all other tests of the package
func getFileNames(t *testing.T, c *S3Client, bucket string) (root *fs.DirectoryInfo) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GetFileNames(%s) panicked: %v", bucket, r)
		}
	}()

	root, err := c.GetFileNames(bucket, 0)
	require.NoError(t, err)

	return root
}

func TestS3Client_GetFileNames_buildsDirectoryTree(t *testing.T) {
	t.Parallel()
	c := newTestS3Client(t, "")
	bucket := createBucket(t, c, "backmon-tree", map[string]string{
		"root.sql":                  "r",
		"backups/dump-1.sql":        "12345",
		"backups/nested/dump-2.sql": "xyz",
	})

	root := getFileNames(t, c, bucket)

	require.Len(t, root.Files, 1)
	assert.Equal(t, "root.sql", root.Files[0].Name)
	assert.Equal(t, "", root.Files[0].Parent)
	assert.Equal(t, int64(1), root.Files[0].Size)

	backups := root.SubDirs["backups"]
	require.NotNil(t, backups)
	require.Len(t, backups.Files, 1)
	assert.Equal(t, "dump-1.sql", backups.Files[0].Name)
	assert.Equal(t, "backups", backups.Files[0].Parent)
	assert.Equal(t, int64(5), backups.Files[0].Size)
	assert.WithinDuration(t, time.Now(), backups.Files[0].ModifiedAt, 5*time.Minute)

	nested := backups.SubDirs["nested"]
	require.NotNil(t, nested)
	require.Len(t, nested.Files, 1)
	assert.Equal(t, "dump-2.sql", nested.Files[0].Name)
	assert.Equal(t, "backups/nested", nested.Files[0].Parent)
}

func TestS3Client_GetFileNames_followsPagination(t *testing.T) {
	t.Parallel()
	c := newTestS3Client(t, "")

	// S3 returns at most 1000 keys per ListObjectsV2 call
	const count = 1005
	objects := make(map[string]string, count)
	for i := range count {
		objects[fmt.Sprintf("many/file-%04d.sql", i)] = "x"
	}
	bucket := createBucket(t, c, "backmon-paging", objects)

	root := getFileNames(t, c, bucket)

	require.NotNil(t, root.SubDirs["many"])
	assert.Len(t, root.SubDirs["many"].Files, count)
}

func TestS3Client_GetFileNames_appliesDotStatFiles(t *testing.T) {
	t.Parallel()
	c := newTestS3Client(t, "")

	stat := "born_at: \"1700000000\"\nmodified_at: \"1700000100\"\narchived_at: \"1700000200\"\n"
	cases := map[string]string{
		"in a directory":     "backups/",
		"in the bucket root": "",
	}

	for name, dir := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bucket := createBucket(t, c, "backmon-stat", map[string]string{
				dir + "dump.sql":      "data",
				dir + "dump.sql.stat": stat,
			})

			root := getFileNames(t, c, bucket)

			files := root.Files
			if dir != "" {
				require.NotNil(t, root.SubDirs["backups"])
				files = root.SubDirs["backups"].Files
			}

			// the .stat file itself is not listed, its values are applied to dump.sql
			require.Len(t, files, 1)
			assert.Equal(t, "dump.sql", files[0].Name)
			assert.Equal(t, time.Unix(1700000000, 0), files[0].BornAt)
			assert.Equal(t, time.Unix(1700000100, 0), files[0].ModifiedAt)
			assert.Equal(t, time.Unix(1700000200, 0), files[0].ArchivedAt)
		})
	}
}

func TestS3Client_Download(t *testing.T) {
	t.Parallel()
	c := newTestS3Client(t, "")
	bucket := createBucket(t, c, "backmon-download", map[string]string{
		"root.sql":         "root content",
		"backups/dump.sql": "nested content",
	})

	cases := map[string]*fs.FileInfo{
		"root content":   {Name: "root.sql", Parent: ""},
		"nested content": {Name: "dump.sql", Parent: "backups"},
	}

	for expected, file := range cases {
		body, length, _, err := c.Download(bucket, file)
		require.NoError(t, err, "download %s/%s", file.Parent, file.Name)

		content, err := io.ReadAll(body)
		_ = body.Close()
		require.NoError(t, err)

		assert.Equal(t, expected, string(content))
		assert.Equal(t, int64(len(expected)), length)
	}
}

func TestS3Client_Delete(t *testing.T) {
	t.Parallel()
	c := newTestS3Client(t, "")

	cases := map[string]*fs.FileInfo{
		"backups/dump.sql": {Name: "dump.sql", Parent: "backups"},
		"root.sql":         {Name: "root.sql", Parent: ""},
	}

	for key, file := range cases {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			bucket := createBucket(t, c, "backmon-delete", map[string]string{key: "x", "keep.sql": "y"})

			require.NoError(t, c.Delete(bucket, file))

			assert.False(t, objectExists(t, c, bucket, key), "%s should have been deleted", key)
			assert.True(t, objectExists(t, c, bucket, "keep.sql"), "other objects must be kept")
		})
	}
}

func TestS3Client_GetDiskNames_autoDiscovery(t *testing.T) {
	t.Parallel()
	c := newTestS3Client(t, "exclude:\n  - \"/^backmon-excluded-/\"\n")
	included := createBucket(t, c, "backmon-included", nil)
	excluded := createBucket(t, c, "backmon-excluded", nil)

	names, err := c.GetDiskNames()
	require.NoError(t, err)

	assert.Contains(t, names, included)
	assert.NotContains(t, names, excluded)
}
