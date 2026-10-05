package provider

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	cfg "github.com/dreitier/backmon/config"
	fs "github.com/dreitier/backmon/storage/fs"
)

// getClient returns an error when no region is configured.
func TestS3Client_GetClientRequiresRegion(t *testing.T) {
	c := &S3Client{AccessKey: "key", SecretKey: "secret"}

	client, err := getClient(c)

	if err == nil {
		t.Fatal("expected an error when no region is set")
	}
	if client != nil {
		t.Errorf("expected a nil client on error, got %v", client)
	}
}

// getClient returns an already-initialised client without rebuilding it.
func TestS3Client_GetClientReusesExistingClient(t *testing.T) {
	// a client with static credentials builds a client lazily, no network traffic
	c := &S3Client{AccessKey: "key", SecretKey: "secret", Region: "eu-central-1"}

	first, err := getClient(c)
	if err != nil {
		t.Fatalf("unexpected error building client: %s", err)
	}
	if first == nil {
		t.Fatal("expected a non-nil client")
	}

	second, err := getClient(c)
	if err != nil {
		t.Fatalf("unexpected error on second call: %s", err)
	}
	if first != second {
		t.Error("expected getClient to return the cached client instance")
	}
}

// getClient honours the endpoint/path-style/TLS-skip options when building a client.
func TestS3Client_GetClientWithEndpointAndOptions(t *testing.T) {
	c := &S3Client{
		AccessKey:      "key",
		SecretKey:      "secret",
		Region:         "eu-central-1",
		Endpoint:       "http://localhost:9000",
		ForcePathStyle: true,
		TLSSkipVerify:  true,
	}

	client, err := getClient(c)
	if err != nil {
		t.Fatalf("unexpected error building client: %s", err)
	}
	if client == nil {
		t.Fatal("expected a non-nil client")
	}
}

// appendFilesTo builds a nested directory tree from flat S3 object keys.
func TestS3Client_AppendFilesToBuildsTree(t *testing.T) {
	c := &S3Client{Name: "test"}
	disk := "bucket"
	root := &fs.DirectoryInfo{Name: disk, SubDirs: make(map[string]*fs.DirectoryInfo)}
	dotStatFiles := make(map[string]string)

	now := time.Now()
	objects := []types.Object{
		{Key: aws.String("top.txt"), LastModified: &now, Size: aws.Int64(10)},
		{Key: aws.String("sub/nested.txt"), LastModified: &now, Size: aws.Int64(20)},
		{Key: aws.String("sub/deeper/leaf.txt"), LastModified: &now, Size: aws.Int64(30)},
	}

	c.appendFilesTo(&disk, root, objects, &dotStatFiles)

	// top-level file sits directly on the root
	if len(root.Files) != 1 || root.Files[0].Name != "top.txt" {
		t.Fatalf("expected one top-level file top.txt, got %+v", root.Files)
	}
	if root.Files[0].Size != 10 {
		t.Errorf("expected size 10, got %d", root.Files[0].Size)
	}

	sub, ok := root.SubDirs["sub"]
	if !ok {
		t.Fatal("expected a 'sub' subdirectory")
	}
	if len(sub.Files) != 1 || sub.Files[0].Name != "nested.txt" {
		t.Fatalf("expected nested.txt under sub, got %+v", sub.Files)
	}
	if sub.Files[0].Parent != "sub" {
		t.Errorf("expected parent 'sub', got %q", sub.Files[0].Parent)
	}

	deeper, ok := sub.SubDirs["deeper"]
	if !ok {
		t.Fatal("expected a 'sub/deeper' subdirectory")
	}
	if len(deeper.Files) != 1 || deeper.Files[0].Name != "leaf.txt" {
		t.Fatalf("expected leaf.txt under sub/deeper, got %+v", deeper.Files)
	}
	if deeper.Files[0].Parent != "sub/deeper" {
		t.Errorf("expected parent 'sub/deeper', got %q", deeper.Files[0].Parent)
	}

	// no .stat files were present
	if len(dotStatFiles) != 0 {
		t.Errorf("expected no .stat files registered, got %v", dotStatFiles)
	}
}

// cleanupTemporaryFiles removes every registered temp file and tolerates missing ones.
func TestS3Client_CleanupTemporaryFiles(t *testing.T) {
	c := &S3Client{}

	tmp, err := os.CreateTemp(t.TempDir(), "backmon-cleanup-")
	if err != nil {
		t.Fatalf("could not create temp file: %s", err)
	}
	_ = tmp.Close()

	missing := filepath.Join(t.TempDir(), "does-not-exist")

	dotStatFiles := map[string]string{
		"regular/file":       tmp.Name(),
		"regular/other-file": missing, // exercises the os.Remove error branch
	}

	c.cleanupTemporaryFiles(&dotStatFiles)

	if _, err := os.Stat(tmp.Name()); !os.IsNotExist(err) {
		t.Errorf("expected temp file %s to be removed", tmp.Name())
	}
}

// hasAccessToBucket short-circuits to false for a disk excluded by policy,
// so it never touches the (nil) client.
func TestS3Client_HasAccessToBucketRejectsExcludedDisk(t *testing.T) {
	raw, _ := cfg.ParseFromString(
		`
exclude:
- secret-bucket
all_others: exclude
`)
	disks := cfg.ParseDisksSection(raw)
	c := &S3Client{Disks: disks}

	name := "secret-bucket"
	if c.hasAccessToBucket(nil, &name) {
		t.Error("expected hasAccessToBucket to reject an excluded disk")
	}

	unknown := "unlisted-bucket"
	if c.hasAccessToBucket(nil, &unknown) {
		t.Error("expected hasAccessToBucket to reject a disk excluded by the all_others policy")
	}
}

// findAvailableDisksByInclusion returns no disks when the include set is empty.
func TestS3Client_FindAvailableDisksByInclusionEmpty(t *testing.T) {
	raw, _ := cfg.ParseFromString("all_others: exclude\n")
	disks := cfg.ParseDisksSection(raw)
	c := &S3Client{Disks: disks}

	result, err := c.findAvailableDisksByInclusion(nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(result) != 0 {
		t.Errorf("expected no included disks, got %v", result)
	}
}
