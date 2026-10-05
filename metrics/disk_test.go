package metrics

import (
	"testing"
	"time"

	fs "github.com/dreitier/backmon/storage/fs"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// NewDisk should bump the global disksTotal gauge, and Drop should bring it
// back down again.
func TestNewDiskAndDropAdjustDisksTotal(t *testing.T) {
	before := testutil.ToFloat64(GetApplicationMetrics().disksTotal)

	disk := NewDisk("disk-total-test")
	if got := testutil.ToFloat64(GetApplicationMetrics().disksTotal); got != before+1 {
		t.Fatalf("after NewDisk disksTotal = %v, want %v", got, before+1)
	}

	disk.Drop()
	if got := testutil.ToFloat64(GetApplicationMetrics().disksTotal); got != before {
		t.Fatalf("after Drop disksTotal = %v, want %v", got, before)
	}
}

func TestUpdateFileLimits(t *testing.T) {
	disk := NewDisk("disk-limits-test")
	defer disk.Drop()

	ctime := time.Unix(1_600_000_000, 0)
	disk.UpdateFileLimits("dir", "file", 7, 90*time.Second, ctime)

	if got := testutil.ToFloat64(disk.fileCountExpected.WithLabelValues("dir", "file")); got != 7 {
		t.Errorf("fileCountExpected = %v, want 7", got)
	}
	if got := testutil.ToFloat64(disk.fileAgeThreshold.WithLabelValues("dir", "file")); got != 90 {
		t.Errorf("fileAgeThreshold = %v, want 90", got)
	}
	if got := testutil.ToFloat64(disk.latestFileCreationExpectedAt.WithLabelValues("dir", "file")); got != float64(ctime.Unix()) {
		t.Errorf("latestFileCreationExpectedAt = %v, want %v", got, float64(ctime.Unix()))
	}
}

func TestUpdateUsageStats(t *testing.T) {
	disk := NewDisk("disk-usage-test")
	defer disk.Drop()

	disk.UpdateUsageStats(42, 123456)

	if got := testutil.ToFloat64(disk.fileCountTotal); got != 42 {
		t.Errorf("fileCountTotal = %v, want 42", got)
	}
	if got := testutil.ToFloat64(disk.diskUsageTotal); got != 123456 {
		t.Errorf("diskUsageTotal = %v, want 123456", got)
	}
}

func TestUpdateFileCounts(t *testing.T) {
	disk := NewDisk("disk-counts-test")
	defer disk.Drop()

	disk.UpdateFileCounts("dir", "file", "group", 5, 3)

	if got := testutil.ToFloat64(disk.fileCount.WithLabelValues("dir", "file", "group")); got != 5 {
		t.Errorf("fileCount = %v, want 5", got)
	}
	if got := testutil.ToFloat64(disk.fileYoungCount.WithLabelValues("dir", "file", "group")); got != 3 {
		t.Errorf("fileYoungCount = %v, want 3", got)
	}
}

// When a group has no files present, UpdateFileCounts must remove the stale
// "latest file" series so Prometheus does not keep exposing an old timestamp.
func TestUpdateFileCountsZeroDropsLatestFileSeries(t *testing.T) {
	disk := NewDisk("disk-counts-zero-test")
	defer disk.Drop()

	info := &fs.FileInfo{
		Size:       10,
		BornAt:     time.Unix(100, 0),
		ModifiedAt: time.Unix(130, 0),
		ArchivedAt: time.Unix(140, 0),
	}
	disk.UpdateLatestFile("dir", "file", "group", info, time.Unix(130, 0))

	if n := testutil.CollectAndCount(disk.latestFileCreatedAt); n != 1 {
		t.Fatalf("expected latestFileCreatedAt to have 1 series, got %d", n)
	}

	disk.UpdateFileCounts("dir", "file", "group", 0, 0)

	if n := testutil.CollectAndCount(disk.latestFileCreatedAt); n != 0 {
		t.Errorf("expected latestFileCreatedAt series to be dropped, got %d", n)
	}
}

func TestUpdateLatestFileComputesDuration(t *testing.T) {
	disk := NewDisk("disk-latest-test")
	defer disk.Drop()

	info := &fs.FileInfo{
		Size:       2048,
		BornAt:     time.Unix(1000, 0),
		ModifiedAt: time.Unix(1075, 0),
		ArchivedAt: time.Unix(1100, 0),
	}
	created := time.Unix(1075, 0)
	disk.UpdateLatestFile("dir", "file", "group", info, created)

	if got := testutil.ToFloat64(disk.latestFileCreatedAt.WithLabelValues("dir", "file", "group")); got != float64(created.Unix()) {
		t.Errorf("latestFileCreatedAt = %v, want %v", got, float64(created.Unix()))
	}
	// duration = ModifiedAt - BornAt = 1075 - 1000 = 75
	if got := testutil.ToFloat64(disk.latestFileCreationDuration.WithLabelValues("dir", "file", "group")); got != 75 {
		t.Errorf("latestFileCreationDuration = %v, want 75", got)
	}
	if got := testutil.ToFloat64(disk.latestFileBornAt.WithLabelValues("dir", "file", "group")); got != 1000 {
		t.Errorf("latestFileBornAt = %v, want 1000", got)
	}
	if got := testutil.ToFloat64(disk.latestFileModifiedAt.WithLabelValues("dir", "file", "group")); got != 1075 {
		t.Errorf("latestFileModifiedAt = %v, want 1075", got)
	}
	if got := testutil.ToFloat64(disk.latestFileArchivedAt.WithLabelValues("dir", "file", "group")); got != 1100 {
		t.Errorf("latestFileArchivedAt = %v, want 1100", got)
	}
	if got := testutil.ToFloat64(disk.latestSize.WithLabelValues("dir", "file", "group")); got != 2048 {
		t.Errorf("latestSize = %v, want 2048", got)
	}
}

// DropFile should remove every per-file series for the given labels.
func TestDropFile(t *testing.T) {
	disk := NewDisk("disk-dropfile-test")
	defer disk.Drop()

	info := &fs.FileInfo{Size: 1, BornAt: time.Unix(1, 0), ModifiedAt: time.Unix(2, 0), ArchivedAt: time.Unix(3, 0)}
	disk.UpdateFileCounts("dir", "file", "group", 4, 2)
	disk.UpdateLatestFile("dir", "file", "group", info, time.Unix(2, 0))

	disk.DropFile("dir", "file", "group")

	if n := testutil.CollectAndCount(disk.fileCount); n != 0 {
		t.Errorf("expected fileCount series dropped, got %d", n)
	}
	if n := testutil.CollectAndCount(disk.latestSize); n != 0 {
		t.Errorf("expected latestSize series dropped, got %d", n)
	}
}

// DefinitionsUpdated flips status to 1 and resets per-file metrics;
// DefinitionsMissing flips status to 0.
func TestDefinitionsStatusTransitions(t *testing.T) {
	disk := NewDisk("disk-status-test")
	defer disk.Drop()

	disk.UpdateFileCounts("dir", "file", "group", 9, 9)

	disk.DefinitionsUpdated()
	if got := testutil.ToFloat64(disk.status); got != 1 {
		t.Errorf("status after DefinitionsUpdated = %v, want 1", got)
	}
	if n := testutil.CollectAndCount(disk.fileCount); n != 0 {
		t.Errorf("expected fileCount reset by DefinitionsUpdated, got %d", n)
	}

	disk.DefinitionsMissing()
	if got := testutil.ToFloat64(disk.status); got != 0 {
		t.Errorf("status after DefinitionsMissing = %v, want 0", got)
	}
}

// UpdateDiskQuota registers and sets the quota gauge when positive and
// unregisters it when zero.
func TestUpdateDiskQuota(t *testing.T) {
	disk := NewDisk("disk-quota-test")
	defer disk.Drop()

	disk.UpdateDiskQuota(500)
	if got := testutil.ToFloat64(disk.diskQuota); got != 500 {
		t.Errorf("diskQuota = %v, want 500", got)
	}

	// Setting the quota again should not error and should overwrite the value.
	disk.UpdateDiskQuota(750)
	if got := testutil.ToFloat64(disk.diskQuota); got != 750 {
		t.Errorf("diskQuota after re-set = %v, want 750", got)
	}

	// A positive quota is registered with the shared registry, so it is
	// exposed when the registry is gathered.
	if n, err := testutil.GatherAndCount(registry, "backmon_disk_quota_bytes"); err != nil || n != 1 {
		t.Errorf("expected disk_quota_bytes exposed by registry, got %d (err=%v)", n, err)
	}

	// Zero quota unregisters the gauge from the registry.
	disk.UpdateDiskQuota(0)
	if n, err := testutil.GatherAndCount(registry, "backmon_disk_quota_bytes"); err != nil || n != 0 {
		t.Errorf("expected disk_quota_bytes unregistered, got %d (err=%v)", n, err)
	}
}
