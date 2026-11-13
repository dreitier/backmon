package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	cfg "github.com/dreitier/backmon/config"
	fs "github.com/dreitier/backmon/storage/fs"
	dotstat "github.com/dreitier/backmon/storage/fs/dotstat"
)

const (
	gcsDefaultEndpoint = "https://storage.googleapis.com"
	// read-write is required for purging; what backmon is actually allowed to do is up to the IAM roles of the principal
	gcsScope = "https://www.googleapis.com/auth/devstorage.read_write"
	// a request is sent at most this many times if the server is unavailable or rate limits
	gcsMaxAttempts = 4
	gcsRetryDelay  = 500 * time.Millisecond
	// upper limit for the part of an error response that is read
	gcsMaxErrorBody = 64 * 1024
)

// GCSClient accesses Google Cloud Storage through its JSON API; each bucket is a disk
type GCSClient struct {
	EnvName string
	// ProjectID is the project whose buckets are listed during auto-discovery; empty means the project of the credentials
	ProjectID string
	// CredentialsFile is the path to a service account key file; without it and CredentialsJSON, Application Default
	// Credentials are used
	CredentialsFile string
	// CredentialsJSON is the content of a service account key file
	CredentialsJSON string
	// Anonymous sends requests without authentication, e.g. for public buckets or emulators
	Anonymous bool
	// Endpoint replaces https://storage.googleapis.com, e.g. for emulators
	Endpoint string
	// Proxy is the URL of an HTTP proxy for all requests of this client; empty means the proxy of the process environment
	Proxy             string
	AutoDiscoverDisks bool
	Disks             *cfg.DisksConfiguration

	// guards the lazy creation of httpClient: disk updates and API requests (e.g. downloads) run concurrently
	clientMu   sync.Mutex
	httpClient *http.Client
	// project used for auto-discovery, resolved together with httpClient
	resolvedProjectID string
	// delay before the first retry, doubled for each further one; zero means gcsRetryDelay
	retryDelay time.Duration
}

// gcsError is returned for responses with an unexpected status code
type gcsError struct {
	StatusCode int
	Message    string
}

func (e *gcsError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("unexpected status %d %s", e.StatusCode, http.StatusText(e.StatusCode))
	}

	return fmt.Sprintf("unexpected status %d %s: %s", e.StatusCode, http.StatusText(e.StatusCode), e.Message)
}

type gcsObject struct {
	Name string `json:"name"`
	// the JSON API encodes 64-bit integers as strings
	Size        json.Number `json:"size"`
	TimeCreated time.Time   `json:"timeCreated"`
	Updated     time.Time   `json:"updated"`
}

type gcsObjectList struct {
	Items         []gcsObject `json:"items"`
	NextPageToken string      `json:"nextPageToken"`
}

type gcsBucketList struct {
	Items []struct {
		Name string `json:"name"`
	} `json:"items"`
	NextPageToken string `json:"nextPageToken"`
}

// client returns the HTTP client which authenticates all requests; it is created on first use
func (c *GCSClient) client() (*http.Client, error) {
	c.clientMu.Lock()
	defer c.clientMu.Unlock()

	if c.httpClient != nil {
		return c.httpClient, nil
	}

	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("default HTTP transport has been replaced")
	}

	transport = transport.Clone()
	// downloads are streamed, so there can't be a timeout for the whole request
	transport.ResponseHeaderTimeout = time.Minute

	if c.Proxy != "" {
		proxyURL, err := url.Parse(c.Proxy)

		if err != nil || proxyURL.Host == "" {
			return nil, fmt.Errorf("invalid proxy URL for environment %#q", c.EnvName)
		}

		log.Debugf("Using proxy %s for environment %#q", proxyURL.Redacted(), c.EnvName)
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	plainClient := &http.Client{Transport: transport}

	if c.Anonymous {
		log.Debugf("Accessing Google Cloud Storage anonymously for environment %#q", c.EnvName)
		c.resolvedProjectID = c.ProjectID
		c.httpClient = plainClient

		return c.httpClient, nil
	}

	// the token requests use the same transport (and thereby proxy) as the storage requests
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, plainClient)
	credentials, err := c.credentials(ctx)

	if err != nil {
		log.Errorf("Unable to load Google Cloud credentials for environment %#q: %v", c.EnvName, err)
		return nil, err
	}

	c.resolvedProjectID = c.ProjectID

	if c.resolvedProjectID == "" {
		c.resolvedProjectID = credentials.ProjectID
	}

	c.httpClient = oauth2.NewClient(ctx, credentials.TokenSource)

	return c.httpClient, nil
}

func (c *GCSClient) credentials(ctx context.Context) (*google.Credentials, error) {
	var key []byte

	switch {
	case c.CredentialsJSON != "" && c.CredentialsFile != "":
		return nil, errors.New("credentials file and credentials JSON are mutually exclusive")
	case c.CredentialsJSON != "":
		key = []byte(c.CredentialsJSON)
	case c.CredentialsFile != "":
		var err error
		key, err = os.ReadFile(c.CredentialsFile)

		if err != nil {
			return nil, fmt.Errorf("unable to read credentials file: %w", err)
		}
	default:
		log.Debug("No service account key provided, trying to use Application Default Credentials.")
		credentials, err := google.FindDefaultCredentials(ctx, gcsScope)

		if err != nil {
			return nil, fmt.Errorf("unable to find Application Default Credentials: %w", err)
		}

		return credentials, nil
	}

	// Only service account keys are accepted here: other credential types (e.g. workload identity federation)
	// reference URLs and files and have to be set up as Application Default Credentials.
	credentials, err := google.CredentialsFromJSONWithType(ctx, key, google.ServiceAccount, gcsScope)

	if err != nil {
		return nil, fmt.Errorf("unable to use service account key: %w", err)
	}

	return credentials, nil
}

// apiURL returns a URL of the JSON API, e.g. apiURL("b", bucket, "o", object) for an object
func (c *GCSClient) apiURL(pathSegments ...string) string {
	endpoint := c.Endpoint

	if endpoint == "" {
		endpoint = gcsDefaultEndpoint
	}

	var b strings.Builder
	b.WriteString(strings.TrimSuffix(endpoint, "/"))
	b.WriteString("/storage/v1")

	for i, segment := range pathSegments {
		b.WriteString("/")

		// collection names are at even positions, names of buckets and objects (which may contain slashes) at odd ones
		if i%2 == 0 {
			b.WriteString(segment)
		} else {
			b.WriteString(url.PathEscape(segment))
		}
	}

	return b.String()
}

func gcsIsRetryable(statusCode int) bool {
	switch statusCode {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}

	return false
}

// do sends a request without body and returns the response if its status is 2xx; the caller has to close its body.
// All requests of backmon are idempotent, so they are repeated if the server is unavailable or rate limits.
func (c *GCSClient) do(method string, rawURL string, query url.Values, header http.Header) (*http.Response, error) {
	client, err := c.client()

	if err != nil {
		return nil, fmt.Errorf("could not acquire GCS client instance: %w", err)
	}

	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}

	delay := c.retryDelay

	if delay == 0 {
		delay = gcsRetryDelay
	}

	var lastErr error

	for attempt := 1; attempt <= gcsMaxAttempts; attempt++ {
		if attempt > 1 {
			log.Debugf("Retrying %s %s (attempt %d of %d): %v", method, rawURL, attempt, gcsMaxAttempts, lastErr)
			time.Sleep(delay)
			delay *= 2
		}

		req, err := http.NewRequestWithContext(context.Background(), method, rawURL, nil)

		if err != nil {
			return nil, err
		}

		for name, values := range header {
			req.Header[name] = values
		}

		resp, err := client.Do(req)

		if err != nil {
			lastErr = err

			// the token endpoint has rejected the credentials, which won't change with another attempt
			var tokenErr *oauth2.RetrieveError

			if errors.As(err, &tokenErr) && tokenErr.Response != nil && !gcsIsRetryable(tokenErr.Response.StatusCode) {
				break
			}

			// e.g. connection resets
			continue
		}

		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			return resp, nil
		}

		lastErr = gcsErrorFrom(resp)
		_ = resp.Body.Close()

		if !gcsIsRetryable(resp.StatusCode) {
			break
		}
	}

	return nil, lastErr
}

// gcsErrorFrom reads the error message of a failed response; the body is not closed
func gcsErrorFrom(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, gcsMaxErrorBody))

	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	message := ""

	if json.Unmarshal(body, &parsed) == nil {
		message = parsed.Error.Message
	}

	return &gcsError{StatusCode: resp.StatusCode, Message: message}
}

// getJSON sends a GET request and decodes the JSON response into result
func (c *GCSClient) getJSON(rawURL string, query url.Values, result interface{}) error {
	resp, err := c.do(http.MethodGet, rawURL, query, nil)

	if err != nil {
		return err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("unable to decode response: %w", err)
	}

	return nil
}

// GetFileNames lists all objects of the bucket; maxDepth is not used, as a bucket is always listed completely
func (c *GCSClient) GetFileNames(diskName string, maxDepth uint64) (*fs.DirectoryInfo, error) {
	bucketRoot := &fs.DirectoryInfo{
		Name:    diskName,
		SubDirs: make(map[string]*fs.DirectoryInfo),
	}

	dotStatFiles := make(map[string] /* path to regular file*/ string /* path to .stat file */)
	// temporary files are removed in any case, also if a later page can't be retrieved
	defer gcsRemoveTemporaryFiles(dotStatFiles)

	query := url.Values{}
	query.Set("fields", "items(name,size,timeCreated,updated),nextPageToken")

	for {
		var page gcsObjectList

		if err := c.getJSON(c.apiURL("b", diskName, "o"), query, &page); err != nil {
			return nil, fmt.Errorf("failed to get objects in disk %#q: %w", diskName, err)
		}

		log.Infof("Retrieved %d items from disk %#q", len(page.Items), diskName)

		c.appendFilesTo(diskName, bucketRoot, page.Items, dotStatFiles)

		if page.NextPageToken == "" {
			break
		}

		query.Set("pageToken", page.NextPageToken)
	}

	dotstat.ApplyDotStatValuesRecursively(dotStatFiles, bucketRoot)

	return bucketRoot, nil
}

// Clean up files which have been temporarily downloaded for .stat introspection
func gcsRemoveTemporaryFiles(dotStatFiles map[string] /* path to regular file*/ string /* path to .stat file */) {
	for _, pathToDotStatFile := range dotStatFiles {
		log.Debugf("Removing temporary file %s", pathToDotStatFile)

		if err := os.Remove(pathToDotStatFile); err != nil {
			log.Errorf("Unable to remove temporary file %s: %s", pathToDotStatFile, err)
		}
	}
}

func (c *GCSClient) appendFilesTo(diskName string, root *fs.DirectoryInfo, objects []gcsObject, dotStatFiles map[string] /* path to regular file*/ string /* path to .stat file */) {
	for _, obj := range objects {
		// zero-byte placeholders like `backups/` are created by the Cloud Console for "folders"; they are no files
		if obj.Name == "" || strings.HasSuffix(obj.Name, "/") {
			continue
		}

		pathSegments := strings.Split(obj.Name, "/")
		fileName := pathSegments[len(pathSegments)-1]
		pathSegments = pathSegments[0 : len(pathSegments)-1]
		currentDir := root

		for _, segment := range pathSegments {
			next := currentDir.SubDirs[segment]

			if next == nil {
				next = &fs.DirectoryInfo{
					Name:    segment,
					SubDirs: make(map[string]*fs.DirectoryInfo),
				}

				currentDir.SubDirs[segment] = next
			}

			currentDir = next
		}

		parentPath := strings.Join(pathSegments, "/")

		// if object is a .stat file, it is downloaded for later introspection
		if dotstat.IsStatFile(fileName) {
			// the key must match `Parent + "/" + Name` of the regular file, see dotstat.ApplyDotStatValues
			pathToNonStatFile := dotstat.RemoveDotStatSuffix(parentPath + "/" + fileName)

			localAbsolutePath, err := c.downloadToTempFile(diskName, obj.Name, parentPath)

			if err != nil {
				log.Errorf("Unable to download .stat file %s: %s", obj.Name, err)
				continue
			}

			// .stat files are registered for later examination
			log.Debugf("Downloaded .stat file %s for %s to local path %s", obj.Name, pathToNonStatFile, localAbsolutePath)
			dotStatFiles[pathToNonStatFile] = localAbsolutePath

			continue
		}

		size, err := obj.Size.Int64()

		if err != nil {
			log.Debugf("Object %s in disk %#q has no valid size (%q)", obj.Name, diskName, obj.Size.String())
			size = 0
		}

		// Objects are immutable: `timeCreated` is the time at which this content has been written. `updated` also
		// changes with the object's metadata (e.g. its storage class), so it would make old backups look new.
		createdAt := obj.TimeCreated

		if createdAt.IsZero() {
			createdAt = obj.Updated
		}

		file := &fs.FileInfo{
			Name:       fileName,
			Parent:     parentPath,
			BornAt:     createdAt,
			ModifiedAt: createdAt,
			ArchivedAt: createdAt,
			Size:       size,
		}

		currentDir.Files = append(currentDir.Files, file)
	}
}

// downloadToTempFile downloads the given object into a temporary file and returns the file's path
func (c *GCSClient) downloadToTempFile(diskName string, objectName string, parentPath string) (string, error) {
	resp, err := c.get(diskName, objectName)

	if err != nil {
		return "", err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	tempFile, err := os.CreateTemp(os.TempDir(), "backmon_"+strings.ReplaceAll(strings.ReplaceAll(parentPath, "/", "_"), "\\", "_"))

	if err != nil {
		return "", fmt.Errorf("unable to create temporary file: %w", err)
	}

	_, err = io.Copy(tempFile, resp.Body)
	closeErr := tempFile.Close()

	if err == nil {
		err = closeErr
	}

	if err != nil {
		_ = os.Remove(tempFile.Name())
		return "", fmt.Errorf("unable to write temporary file: %w", err)
	}

	return tempFile.Name(), nil
}

// get requests the content of an object
func (c *GCSClient) get(diskName string, objectName string) (*http.Response, error) {
	query := url.Values{}
	query.Set("alt", "media")

	// Objects stored with `Content-Encoding: gzip` would be decompressed on the fly by Cloud Storage, without a
	// content length. Accepting gzip returns the object as it is stored, which is what backmon monitors.
	header := http.Header{}
	header.Set("Accept-Encoding", "gzip")

	return c.do(http.MethodGet, c.apiURL("b", diskName, "o", objectName), query, header)
}

func (c *GCSClient) GetDiskNames() ([]string, error) {
	if c.AutoDiscoverDisks {
		return c.findAvailableDisksByAutoDiscovery()
	}

	return c.findAvailableDisksByInclusion()
}

// Find available disks by iterating over each bucket of the project. That assumes that the principal has the IAM
// permission `storage.buckets.list` in that project
func (c *GCSClient) findAvailableDisksByAutoDiscovery() ([]string, error) {
	var r []string

	// resolves the project of the credentials
	if _, err := c.client(); err != nil {
		return nil, fmt.Errorf("could not acquire GCS client instance: %w", err)
	}

	if c.resolvedProjectID == "" {
		return nil, fmt.Errorf("failed to list GCS disks by auto discovery: no project_id configured for environment %#q and the credentials don't contain one", c.EnvName)
	}

	log.Infof("Auto-discovering disks based upon available GCS buckets of project %s...", c.resolvedProjectID)

	query := url.Values{}
	query.Set("project", c.resolvedProjectID)
	query.Set("fields", "items(name),nextPageToken")

	for {
		var page gcsBucketList

		if err := c.getJSON(c.apiURL("b"), query, &page); err != nil {
			return nil, fmt.Errorf("failed to list GCS disks by auto discovery: %w", err)
		}

		for _, bucketAsDisk := range page.Items {
			log.Infof("Discovered bucket %s", bucketAsDisk.Name)

			if c.hasAccessToBucket(bucketAsDisk.Name) {
				r = append(r, bucketAsDisk.Name)
			} else {
				log.Warnf("Don't have access to bucket %s, discarding", bucketAsDisk.Name)
			}
		}

		if page.NextPageToken == "" {
			break
		}

		query.Set("pageToken", page.NextPageToken)
	}

	return r, nil
}

// Find available disks by iterating over disks.include configuration parameter
func (c *GCSClient) findAvailableDisksByInclusion() ([]string, error) {
	var r []string

	log.Info("Finding disks based upon disks.include configuration parameter...")

	for keyAsBucketName := range c.Disks.GetIncludedDisks() {
		if c.hasAccessToBucket(keyAsBucketName) {
			r = append(r, keyAsBucketName)
		}
	}

	return r, nil
}

// Check if objects of the bucket can be listed. It is basically a test for the IAM permission `storage.objects.list`
func (c *GCSClient) hasAccessToBucket(bucketName string) bool {
	// don't try to list items in ignored disks
	if !c.Disks.IsDiskIncluded(bucketName) {
		return false
	}

	query := url.Values{}
	query.Set("maxResults", "1")
	query.Set("fields", "nextPageToken")

	var page gcsObjectList

	if err := c.getJSON(c.apiURL("b", bucketName, "o"), query, &page); err != nil {
		log.Debugf("Unable to list items in GCS bucket %q, %v; won't use it as disk", bucketName, err)
		return false
	}

	return true
}

func (c *GCSClient) Download(disk string, file *fs.FileInfo) (bytes io.ReadCloser, length int64, contentType string, err error) {
	fullName := objectKey(file)

	resp, err := c.get(disk, fullName)

	if err != nil {
		return nil, -1, "", fmt.Errorf("failed to download object %s from disk %s: %w", fullName, disk, err)
	}

	length = resp.ContentLength

	if length < 0 {
		// chunked response: fall back to the stored size, which is the same as objects are never decompressed (see get)
		if stored, parseErr := strconv.ParseInt(resp.Header.Get("X-Goog-Stored-Content-Length"), 10, 64); parseErr == nil {
			length = stored
		} else {
			length = file.Size
		}
	}

	return resp.Body, length, resp.Header.Get("Content-Type"), nil
}

func (c *GCSClient) Delete(disk string, file *fs.FileInfo) error {
	// In buckets with object versioning, this only makes the live version noncurrent; removing noncurrent versions is
	// up to the bucket's lifecycle rules.
	fullName := objectKey(file)

	resp, err := c.do(http.MethodDelete, c.apiURL("b", disk, "o", fullName), nil, nil)

	if err != nil {
		return fmt.Errorf("failed to delete object %s from disk %s: %w", fullName, disk, err)
	}

	_ = resp.Body.Close()

	return nil
}
