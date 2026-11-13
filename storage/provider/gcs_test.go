package provider

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fs "github.com/dreitier/backmon/storage/fs"
)

var (
	testServiceAccountKeyOnce sync.Once
	testServiceAccountKey     string
)

// serviceAccountKey returns the content of a service account key file whose tokens are issued by a local test server
func serviceAccountKey(t *testing.T, credentialType string) string {
	t.Helper()

	// generating an RSA key is slow, one for all tests is sufficient
	testServiceAccountKeyOnce.Do(func() {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		der, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)

		testServiceAccountKey = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	})

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(tokenServer.Close)

	content, err := json.Marshal(map[string]string{
		"type":           credentialType,
		"project_id":     "project-of-key",
		"private_key_id": "test",
		"private_key":    testServiceAccountKey,
		"client_email":   "backmon@project-of-key.iam.gserviceaccount.com",
		"token_uri":      tokenServer.URL,
	})
	require.NoError(t, err)

	return string(content)
}

// newFakeGCSClient returns a fake server and an anonymous client for it
func newFakeGCSClient(t *testing.T) (*fakeGCS, *GCSClient) {
	t.Helper()

	fake := newFakeGCS(t)

	return fake, &GCSClient{
		Anonymous:         true,
		ProjectID:         gcsTestProject,
		Endpoint:          fake.URL,
		EnvName:           "test",
		AutoDiscoverDisks: true,
		Disks:             parseTestDisks(t, ""),
		retryDelay:        time.Millisecond,
	}
}

func TestGCSClient_apiURL(t *testing.T) {
	c := &GCSClient{}
	assert.Equal(t, "https://storage.googleapis.com/storage/v1/b", c.apiURL("b"))
	assert.Equal(t, "https://storage.googleapis.com/storage/v1/b/my-bucket/o", c.apiURL("b", "my-bucket", "o"))
	// slashes of object names must not become path separators
	assert.Equal(t,
		"https://storage.googleapis.com/storage/v1/b/my-bucket/o/backups%2F2026%2Fdump%20%231%3F.sql",
		c.apiURL("b", "my-bucket", "o", "backups/2026/dump #1?.sql"))

	c = &GCSClient{Endpoint: "http://localhost:4443/"}
	assert.Equal(t, "http://localhost:4443/storage/v1/b/my-bucket", c.apiURL("b", "my-bucket"))
}

func TestGCSClient_clientIsReused(t *testing.T) {
	c := &GCSClient{Anonymous: true}

	first, err := c.client()
	require.NoError(t, err)
	second, err := c.client()
	require.NoError(t, err)

	assert.Same(t, first, second)
}

func TestGCSClient_anonymousSendsNoCredentials(t *testing.T) {
	fake, c := newFakeGCSClient(t)
	fake.put("bucket", "dump.sql", "x")

	_, err := c.GetFileNames("bucket", 0)
	require.NoError(t, err)

	assert.Equal(t, []string{""}, fake.authorizations)
}

// A service account key can be given inline or as file; its project is used for auto-discovery
func TestGCSClient_serviceAccountKey(t *testing.T) {
	key := serviceAccountKey(t, "service_account")
	keyFile := filepath.Join(t.TempDir(), "key.json")
	require.NoError(t, os.WriteFile(keyFile, []byte(key), 0o600))

	cases := map[string]*GCSClient{
		"credentials_json": {CredentialsJSON: key},
		"credentials_file": {CredentialsFile: keyFile},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			fake := newFakeGCS(t)
			fake.put("bucket", "dump.sql", "x")

			c.Endpoint = fake.URL
			c.AutoDiscoverDisks = true
			c.Disks = parseTestDisks(t, "")

			names, err := c.GetDiskNames()
			require.NoError(t, err)

			assert.Equal(t, []string{"bucket"}, names)
			assert.Equal(t, []string{"project-of-key"}, fake.projects)
			require.NotEmpty(t, fake.authorizations)
			for _, authorization := range fake.authorizations {
				assert.Equal(t, "Bearer test-token", authorization)
			}
		})
	}
}

func TestGCSClient_configuredProjectOverridesProjectOfKey(t *testing.T) {
	fake := newFakeGCS(t)
	c := &GCSClient{
		CredentialsJSON:   serviceAccountKey(t, "service_account"),
		ProjectID:         "other-project",
		Endpoint:          fake.URL,
		AutoDiscoverDisks: true,
		Disks:             parseTestDisks(t, ""),
	}

	_, err := c.GetDiskNames()
	require.NoError(t, err)

	assert.Equal(t, []string{"other-project"}, fake.projects)
}

// Without a key, Application Default Credentials are used
func TestGCSClient_applicationDefaultCredentials(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "adc.json")
	require.NoError(t, os.WriteFile(keyFile, []byte(serviceAccountKey(t, "service_account")), 0o600))
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", keyFile)

	fake := newFakeGCS(t)
	fake.put("bucket", "dump.sql", "x")
	c := &GCSClient{Endpoint: fake.URL}

	_, err := c.GetFileNames("bucket", 0)
	require.NoError(t, err)

	assert.Equal(t, []string{"Bearer test-token"}, fake.authorizations)
}

func TestGCSClient_applicationDefaultCredentialsNotFound(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", filepath.Join(t.TempDir(), "missing.json"))

	c := &GCSClient{Endpoint: newFakeGCS(t).URL}

	_, err := c.GetFileNames("bucket", 0)
	assert.ErrorContains(t, err, "Application Default Credentials")
}

func TestGCSClient_invalidCredentials(t *testing.T) {
	cases := map[string]*GCSClient{
		"missing file":           {CredentialsFile: filepath.Join(t.TempDir(), "missing.json")},
		"no JSON":                {CredentialsJSON: "not a key"},
		"file and JSON":          {CredentialsFile: "key.json", CredentialsJSON: "{}"},
		"not a service account":  {CredentialsJSON: serviceAccountKey(t, "authorized_user")},
		"external account":       {CredentialsJSON: `{"type":"external_account","audience":"a","token_url":"http://localhost"}`},
		"invalid proxy":          {Anonymous: true, Proxy: "://missing-scheme"},
		"proxy without hostname": {Anonymous: true, Proxy: "http://"},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			fake := newFakeGCS(t)
			c.Endpoint = fake.URL
			c.AutoDiscoverDisks = true
			c.Disks = parseTestDisks(t, "")

			_, err := c.GetDiskNames()
			assert.Error(t, err)

			_, err = c.GetFileNames("bucket", 0)
			assert.Error(t, err)

			_, _, _, err = c.Download("bucket", &fs.FileInfo{Name: "dump.sql"})
			assert.Error(t, err)

			assert.Error(t, c.Delete("bucket", &fs.FileInfo{Name: "dump.sql"}))
			assert.Empty(t, fake.requests, "no request may be sent without valid credentials")
		})
	}
}

func TestGCSClient_GetDiskNames_autoDiscoveryRequiresProject(t *testing.T) {
	fake, c := newFakeGCSClient(t)
	c.ProjectID = ""

	_, err := c.GetDiskNames()

	assert.ErrorContains(t, err, "project_id")
	assert.Empty(t, fake.requests)
}

func TestGCSClient_GetDiskNames_followsPagination(t *testing.T) {
	fake, c := newFakeGCSClient(t)
	fake.pageSize = 2
	for _, bucket := range []string{"bucket-1", "bucket-2", "bucket-3", "bucket-4", "bucket-5"} {
		fake.put(bucket, "dump.sql", "x")
	}

	names, err := c.GetDiskNames()
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"bucket-1", "bucket-2", "bucket-3", "bucket-4", "bucket-5"}, names)
	assert.Equal(t, 3, fake.requestCount("GET /storage/v1/b"))
}

// Buckets whose objects can't be listed are no disks
func TestGCSClient_GetDiskNames_discardsInaccessibleBuckets(t *testing.T) {
	fake, c := newFakeGCSClient(t)
	fake.put("accessible", "dump.sql", "x")
	fake.put("forbidden", "dump.sql", "x")
	fake.denied["forbidden"] = true

	names, err := c.GetDiskNames()
	require.NoError(t, err)

	assert.Equal(t, []string{"accessible"}, names)
}

func TestGCSClient_GetDiskNames_autoDiscoveryFails(t *testing.T) {
	fake, c := newFakeGCSClient(t)
	fake.failures = []int{http.StatusForbidden}

	_, err := c.GetDiskNames()

	var apiErr *gcsError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Contains(t, err.Error(), "injected failure")
}

func TestGCSClient_GetFileNames_followsPaginationOfSmallPages(t *testing.T) {
	fake, c := newFakeGCSClient(t)
	fake.pageSize = 2
	for _, name := range []string{"a.sql", "b.sql", "dir/c.sql", "dir/d.sql", "dir/e.sql"} {
		fake.put("bucket", name, "x")
	}

	root, err := c.GetFileNames("bucket", 0)
	require.NoError(t, err)

	assert.Len(t, root.Files, 2)
	require.NotNil(t, root.SubDirs["dir"])
	assert.Len(t, root.SubDirs["dir"].Files, 3)
	assert.Equal(t, 3, fake.requestCount("GET /storage/v1/b/bucket/o"))
}

// The Cloud Console creates empty objects like `backups/` for folders
func TestGCSClient_GetFileNames_skipsFolderPlaceholders(t *testing.T) {
	fake, c := newFakeGCSClient(t)
	fake.put("bucket", "backups/", "")
	fake.put("bucket", "backups/empty/", "")
	fake.put("bucket", "backups/dump.sql", "x")

	root, err := c.GetFileNames("bucket", 0)
	require.NoError(t, err)

	assert.Empty(t, root.Files)
	require.NotNil(t, root.SubDirs["backups"])
	require.Len(t, root.SubDirs["backups"].Files, 1)
	assert.Equal(t, "dump.sql", root.SubDirs["backups"].Files[0].Name)
	assert.Empty(t, root.SubDirs["backups"].SubDirs)
}

// .stat files are downloaded to temporary files, which must not be left behind
func TestGCSClient_GetFileNames_removesTemporaryFiles(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TMPDIR", tempDir)

	fake, c := newFakeGCSClient(t)
	fake.pageSize = 2
	fake.put("bucket", "backups/dump.sql", "data")
	fake.put("bucket", "backups/dump.sql.stat", "born_at: \"1700000000\"\n")
	fake.put("bucket", "backups/z.sql", "data")

	root, err := c.GetFileNames("bucket", 0)
	require.NoError(t, err)
	require.Len(t, root.SubDirs["backups"].Files, 2)
	assert.Equal(t, time.Unix(1700000000, 0), root.SubDirs["backups"].Files[0].BornAt)

	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.Empty(t, entries)

	// also if the listing fails after a .stat file has been downloaded: first page, .stat file, then the second page
	fake.failAfter(2, http.StatusForbidden)

	_, err = c.GetFileNames("bucket", 0)
	require.Error(t, err)

	entries, err = os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// A .stat file which can't be downloaded is ignored, the regular file keeps the times of the object
func TestGCSClient_GetFileNames_ignoresBrokenDotStatFile(t *testing.T) {
	fake, c := newFakeGCSClient(t)
	fake.put("bucket", "dump.sql", "data")
	fake.put("bucket", "dump.sql.stat", "born_at: \"1700000000\"\n")
	fake.failAfter(1, http.StatusForbidden)

	root, err := c.GetFileNames("bucket", 0)
	require.NoError(t, err)

	require.Len(t, root.Files, 1)
	assert.WithinDuration(t, time.Now(), root.Files[0].BornAt, time.Minute)
}

func TestGCSClient_appendFilesTo_usesTimeOfCreation(t *testing.T) {
	c := &GCSClient{}
	root := &fs.DirectoryInfo{Name: "bucket", SubDirs: make(map[string]*fs.DirectoryInfo)}
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := time.Date(2026, 6, 7, 8, 9, 10, 0, time.UTC)

	c.appendFilesTo("bucket", root, []gcsObject{
		// metadata changes (e.g. of the storage class) must not make a backup look new
		{Name: "created.sql", Size: "42", TimeCreated: created, Updated: updated},
		{Name: "only-updated.sql", Size: "1", Updated: updated},
		{Name: "invalid-size.sql", Size: "many", TimeCreated: created},
		{Name: ""},
	}, map[string]string{})

	require.Len(t, root.Files, 3)

	assert.Equal(t, int64(42), root.Files[0].Size)
	assert.Equal(t, created, root.Files[0].BornAt)
	assert.Equal(t, created, root.Files[0].ModifiedAt)
	assert.Equal(t, created, root.Files[0].ArchivedAt)

	assert.Equal(t, updated, root.Files[1].ModifiedAt)
	assert.Equal(t, int64(0), root.Files[2].Size)
}

// The JSON API encodes sizes as strings; servers answering with numbers are accepted as well
func TestGCSClient_decodesSizes(t *testing.T) {
	for _, body := range []string{`{"items":[{"name":"a","size":"123"}]}`, `{"items":[{"name":"a","size":123}]}`} {
		var list gcsObjectList
		require.NoError(t, json.Unmarshal([]byte(body), &list))

		size, err := list.Items[0].Size.Int64()
		require.NoError(t, err)
		assert.Equal(t, int64(123), size)
	}
}

func TestGCSClient_retriesUnavailableServer(t *testing.T) {
	fake, c := newFakeGCSClient(t)
	fake.put("bucket", "dump.sql", "x")
	// 0 drops the connection
	fake.failures = []int{http.StatusServiceUnavailable, http.StatusTooManyRequests, 0}

	root, err := c.GetFileNames("bucket", 0)
	require.NoError(t, err)

	assert.Len(t, root.Files, 1)
	assert.Equal(t, 4, fake.requestCount("GET /storage/v1/b/bucket/o"))
}

func TestGCSClient_givesUpAfterRetries(t *testing.T) {
	fake, c := newFakeGCSClient(t)
	fake.failures = []int{http.StatusBadGateway, http.StatusBadGateway, http.StatusBadGateway, http.StatusBadGateway, http.StatusBadGateway}

	err := c.Delete("bucket", &fs.FileInfo{Name: "dump.sql", Parent: "backups"})

	var apiErr *gcsError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	assert.Equal(t, gcsMaxAttempts, fake.requestCount("DELETE /storage/v1/b/bucket/o/backups%2Fdump.sql"))
}

func TestGCSClient_doesNotRetryClientErrors(t *testing.T) {
	fake, c := newFakeGCSClient(t)
	fake.failures = []int{http.StatusForbidden}

	_, err := c.GetFileNames("bucket", 0)

	require.Error(t, err)
	assert.Len(t, fake.requests, 1)
}

// Credentials which are rejected by the token endpoint are not tried again
func TestGCSClient_doesNotRetryRejectedCredentials(t *testing.T) {
	tokenRequests := 0
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenRequests++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"Invalid JWT Signature."}`))
	}))
	defer tokenServer.Close()

	var key map[string]string
	require.NoError(t, json.Unmarshal([]byte(serviceAccountKey(t, "service_account")), &key))
	key["token_uri"] = tokenServer.URL
	keyJSON, err := json.Marshal(key)
	require.NoError(t, err)

	fake := newFakeGCS(t)
	c := &GCSClient{CredentialsJSON: string(keyJSON), Endpoint: fake.URL, retryDelay: time.Millisecond}

	_, err = c.GetFileNames("bucket", 0)

	assert.ErrorContains(t, err, "invalid_grant")
	assert.Equal(t, 1, tokenRequests)
	assert.Empty(t, fake.requests)
}

func TestGCSClient_invalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>not the JSON API</html>"))
	}))
	defer server.Close()

	c := &GCSClient{Anonymous: true, Endpoint: server.URL}

	_, err := c.GetFileNames("bucket", 0)
	assert.ErrorContains(t, err, "unable to decode response")
}

func TestGCSError_Error(t *testing.T) {
	assert.Equal(t, "unexpected status 404 Not Found: No such object", (&gcsError{StatusCode: http.StatusNotFound, Message: "No such object"}).Error())
	assert.Equal(t, "unexpected status 502 Bad Gateway", (&gcsError{StatusCode: http.StatusBadGateway}).Error())
}

// Objects are requested as they are stored, also if they have been uploaded with `Content-Encoding: gzip`
func TestGCSClient_Download_requestsStoredObject(t *testing.T) {
	var acceptEncoding string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptEncoding = r.Header.Get("Accept-Encoding")
		w.Header().Set("Content-Type", "application/sql")
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("X-Goog-Stored-Content-Length", "10")
		// without content length: the response is chunked
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte("compressed"))
	}))
	defer server.Close()

	c := &GCSClient{Anonymous: true, Endpoint: server.URL}

	body, length, contentType, err := c.Download("bucket", &fs.FileInfo{Name: "dump.sql.gz", Size: 99})
	require.NoError(t, err)

	content, err := io.ReadAll(body)
	_ = body.Close()
	require.NoError(t, err)

	assert.Equal(t, "gzip", acceptEncoding)
	// not decompressed by the HTTP client
	assert.Equal(t, "compressed", string(content))
	assert.Equal(t, int64(10), length)
	assert.Equal(t, "application/sql", contentType)
}

func TestGCSClient_Download_fallsBackToListedSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte("content"))
	}))
	defer server.Close()

	c := &GCSClient{Anonymous: true, Endpoint: server.URL}

	body, length, _, err := c.Download("bucket", &fs.FileInfo{Name: "dump.sql", Size: 7})
	require.NoError(t, err)
	_ = body.Close()

	assert.Equal(t, int64(7), length)
}

// Requests to Cloud Storage and for tokens must be sent to the configured proxy
func TestGCSClient_sendsRequestsThroughProxy(t *testing.T) {
	var mu sync.Mutex
	var proxiedHosts []string

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		proxiedHosts = append(proxiedHosts, r.URL.Host)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if r.URL.Host == "token.invalid" {
			_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer proxy.Close()

	var key map[string]string
	require.NoError(t, json.Unmarshal([]byte(serviceAccountKey(t, "service_account")), &key))
	// .invalid never resolves, so the requests can only succeed through the proxy
	key["token_uri"] = "http://token.invalid/token"
	keyJSON, err := json.Marshal(key)
	require.NoError(t, err)

	c := &GCSClient{
		Proxy:           proxy.URL,
		CredentialsJSON: string(keyJSON),
		Endpoint:        "http://gcs.invalid",
	}

	_, err = c.GetFileNames("bucket", 0)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"token.invalid", "gcs.invalid"}, proxiedHosts)
}
