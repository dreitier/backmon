package provider

// Tests of the Google Cloud Storage client against a server implementing the JSON API.
// Without BACKMON_TEST_GCS_ENDPOINT they run against an in-memory fake (see gcs_fake_test.go). With it, they are
// integration tests against that server: run them locally with `make test-gcs` (starts fake-gcs-server in Docker),
// CI runs them against a fake-gcs-server container.

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cfg "github.com/dreitier/backmon/config"
	fs "github.com/dreitier/backmon/storage/fs"
)

const (
	gcsEndpointEnv = "BACKMON_TEST_GCS_ENDPOINT"
	gcsTestProject = "backmon-test"
)

func parseTestDisks(t *testing.T, disksYaml string) *cfg.DisksConfiguration {
	t.Helper()

	if disksYaml == "" {
		disksYaml = "{}"
	}
	raw, err := cfg.Parse(strings.NewReader(disksYaml))
	require.NoError(t, err)

	return cfg.ParseDisksSection(raw)
}

// newTestGCSClient returns a client for the test server; disksYaml is the `disks:` section of the environment
func newTestGCSClient(t *testing.T, disksYaml string) *GCSClient {
	t.Helper()

	endpoint := os.Getenv(gcsEndpointEnv)
	if endpoint == "" {
		endpoint = newFakeGCS(t).URL
	}

	return &GCSClient{
		Anonymous:         true,
		ProjectID:         gcsTestProject,
		Endpoint:          strings.TrimSuffix(endpoint, "/"),
		EnvName:           "test",
		AutoDiscoverDisks: true,
		Disks:             parseTestDisks(t, disksYaml),
	}
}

// gcsTestRequest sends a request to the test server and returns the status code
func gcsTestRequest(method string, rawURL string, contentType string, body string) (int, error) {
	req, err := http.NewRequest(method, rawURL, strings.NewReader(body))
	if err != nil {
		return 0, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	return resp.StatusCode, nil
}

// createGCSBucket creates a bucket with a unique name and the given objects (name -> content);
// the bucket is removed when the test has finished
func createGCSBucket(t *testing.T, c *GCSClient, prefix string, objects map[string]string) string {
	t.Helper()

	suffix := make([]byte, 4)
	_, err := rand.Read(suffix)
	require.NoError(t, err)
	bucket := prefix + "-" + hex.EncodeToString(suffix)

	status, err := gcsTestRequest(http.MethodPost, c.Endpoint+"/storage/v1/b?project="+gcsTestProject, "application/json", `{"name":"`+bucket+`"}`)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "create bucket %s", bucket)
	t.Cleanup(func() { removeGCSBucket(t, c, bucket) })

	var wg sync.WaitGroup
	errs := make(chan error, len(objects))
	// limit the number of parallel uploads
	sem := make(chan struct{}, 16)

	for name, content := range objects {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			uploadURL := c.Endpoint + "/upload/storage/v1/b/" + bucket + "/o?uploadType=media&name=" + url.QueryEscape(name)
			status, err := gcsTestRequest(http.MethodPost, uploadURL, "application/octet-stream", content)
			if err == nil && status != http.StatusOK {
				err = fmt.Errorf("status %d", status)
			}
			if err != nil {
				errs <- fmt.Errorf("upload %s: %w", name, err)
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	return bucket
}

func removeGCSBucket(t *testing.T, c *GCSClient, bucket string) {
	root, err := c.GetFileNames(bucket, 0)
	if err != nil {
		t.Logf("cleanup: unable to list objects in %s: %s", bucket, err)
		return
	}

	var remove func(dir *fs.DirectoryInfo)
	remove = func(dir *fs.DirectoryInfo) {
		for _, file := range dir.Files {
			_ = c.Delete(bucket, file)
			// .stat files are not listed
			_ = c.Delete(bucket, &fs.FileInfo{Name: file.Name + ".stat", Parent: file.Parent})
		}
		for _, sub := range dir.SubDirs {
			remove(sub)
		}
	}
	remove(root)

	if status, err := gcsTestRequest(http.MethodDelete, c.apiURL("b", bucket), "", ""); err != nil || status >= http.StatusMultipleChoices {
		t.Logf("cleanup: unable to delete bucket %s: status %d, %v", bucket, status, err)
	}
}

func gcsObjectExists(t *testing.T, c *GCSClient, bucket string, name string) bool {
	t.Helper()

	status, err := gcsTestRequest(http.MethodGet, c.apiURL("b", bucket, "o", name), "", "")
	require.NoError(t, err)

	if status == http.StatusOK {
		return true
	}

	require.Equal(t, http.StatusNotFound, status, "unexpected status for object %s", name)

	return false
}

func TestGCSClient_GetFileNames_buildsDirectoryTree(t *testing.T) {
	t.Parallel()
	c := newTestGCSClient(t, "")
	bucket := createGCSBucket(t, c, "backmon-tree", map[string]string{
		"root.sql":                  "r",
		"backups/dump-1.sql":        "12345",
		"backups/nested/dump-2.sql": "xyz",
	})

	root, err := c.GetFileNames(bucket, 0)
	require.NoError(t, err)

	assert.Equal(t, bucket, root.Name)
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
	assert.WithinDuration(t, time.Now(), backups.Files[0].BornAt, 5*time.Minute)
	assert.WithinDuration(t, time.Now(), backups.Files[0].ModifiedAt, 5*time.Minute)
	assert.WithinDuration(t, time.Now(), backups.Files[0].ArchivedAt, 5*time.Minute)

	nested := backups.SubDirs["nested"]
	require.NotNil(t, nested)
	require.Len(t, nested.Files, 1)
	assert.Equal(t, "dump-2.sql", nested.Files[0].Name)
	assert.Equal(t, "backups/nested", nested.Files[0].Parent)
}

func TestGCSClient_GetFileNames_followsPagination(t *testing.T) {
	t.Parallel()
	c := newTestGCSClient(t, "")

	// Cloud Storage returns at most 1000 objects per page
	const count = 1005
	objects := make(map[string]string, count)
	for i := range count {
		objects[fmt.Sprintf("many/file-%04d.sql", i)] = "x"
	}
	bucket := createGCSBucket(t, c, "backmon-paging", objects)

	root, err := c.GetFileNames(bucket, 0)
	require.NoError(t, err)

	require.NotNil(t, root.SubDirs["many"])
	assert.Len(t, root.SubDirs["many"].Files, count)
}

func TestGCSClient_GetFileNames_appliesDotStatFiles(t *testing.T) {
	t.Parallel()
	c := newTestGCSClient(t, "")

	stat := "born_at: \"1700000000\"\nmodified_at: \"1700000100\"\narchived_at: \"1700000200\"\n"
	cases := map[string]string{
		"in a directory":     "backups/",
		"in the bucket root": "",
	}

	for name, dir := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bucket := createGCSBucket(t, c, "backmon-stat", map[string]string{
				dir + "dump.sql":      "data",
				dir + "dump.sql.stat": stat,
			})

			root, err := c.GetFileNames(bucket, 0)
			require.NoError(t, err)

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

func TestGCSClient_GetFileNames_unknownBucket(t *testing.T) {
	t.Parallel()
	c := newTestGCSClient(t, "")

	_, err := c.GetFileNames("backmon-does-not-exist", 0)

	var apiErr *gcsError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
}

func TestGCSClient_Download(t *testing.T) {
	t.Parallel()
	c := newTestGCSClient(t, "")
	bucket := createGCSBucket(t, c, "backmon-download", map[string]string{
		"root.sql":                    "root content",
		"backups/dump.sql":            "nested content",
		"backups/with space & ü.sql":  "special characters",
		"backups/nested/a/b/dump.sql": "deeply nested content",
	})

	cases := map[string]*fs.FileInfo{
		"root content":          {Name: "root.sql", Parent: ""},
		"nested content":        {Name: "dump.sql", Parent: "backups"},
		"special characters":    {Name: "with space & ü.sql", Parent: "backups"},
		"deeply nested content": {Name: "dump.sql", Parent: "backups/nested/a/b"},
	}

	for expected, file := range cases {
		body, length, contentType, err := c.Download(bucket, file)
		require.NoError(t, err, "download %s/%s", file.Parent, file.Name)

		content, err := io.ReadAll(body)
		_ = body.Close()
		require.NoError(t, err)

		assert.Equal(t, expected, string(content))
		assert.Equal(t, int64(len(expected)), length)
		assert.NotEmpty(t, contentType)
	}
}

func TestGCSClient_Download_unknownObject(t *testing.T) {
	t.Parallel()
	c := newTestGCSClient(t, "")
	bucket := createGCSBucket(t, c, "backmon-download", nil)

	_, _, _, err := c.Download(bucket, &fs.FileInfo{Name: "missing.sql", Parent: "backups"})

	var apiErr *gcsError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
}

func TestGCSClient_Delete(t *testing.T) {
	t.Parallel()
	c := newTestGCSClient(t, "")

	cases := map[string]*fs.FileInfo{
		"backups/dump.sql": {Name: "dump.sql", Parent: "backups"},
		"root.sql":         {Name: "root.sql", Parent: ""},
	}

	for name, file := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bucket := createGCSBucket(t, c, "backmon-delete", map[string]string{name: "x", "keep.sql": "y"})

			require.NoError(t, c.Delete(bucket, file))

			assert.False(t, gcsObjectExists(t, c, bucket, name), "%s should have been deleted", name)
			assert.True(t, gcsObjectExists(t, c, bucket, "keep.sql"), "other objects must be kept")
		})
	}
}

func TestGCSClient_Delete_unknownObject(t *testing.T) {
	t.Parallel()
	c := newTestGCSClient(t, "")
	bucket := createGCSBucket(t, c, "backmon-delete", nil)

	err := c.Delete(bucket, &fs.FileInfo{Name: "missing.sql", Parent: ""})

	var apiErr *gcsError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
}

func TestGCSClient_GetDiskNames_autoDiscovery(t *testing.T) {
	t.Parallel()
	c := newTestGCSClient(t, "exclude:\n  - \"/^backmon-excluded-/\"\n")
	included := createGCSBucket(t, c, "backmon-included", nil)
	excluded := createGCSBucket(t, c, "backmon-excluded", nil)

	names, err := c.GetDiskNames()
	require.NoError(t, err)

	assert.Contains(t, names, included)
	assert.NotContains(t, names, excluded)
}

func TestGCSClient_GetDiskNames_byInclusion(t *testing.T) {
	t.Parallel()
	c := newTestGCSClient(t, "")
	included := createGCSBucket(t, c, "backmon-included", nil)
	other := createGCSBucket(t, c, "backmon-other", nil)

	c.AutoDiscoverDisks = false
	c.Disks = parseTestDisks(t, "include:\n  - "+included+"\n  - backmon-does-not-exist\n")

	names, err := c.GetDiskNames()
	require.NoError(t, err)

	// buckets which are not listed in `include` or can't be accessed are no disks
	assert.Equal(t, []string{included}, names)
	assert.NotContains(t, names, other)
}
