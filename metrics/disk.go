package metrics

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"time"

	fs "github.com/dreitier/backmon/storage/fs"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	LabelNameDir   = "dir"
	LabelNameFile  = "file"
	LabelNameGroup = "group"
)

type DiskMetric struct {
	status                       prometheus.Gauge
	fileCountTotal               prometheus.Gauge
	diskUsageTotal               prometheus.Gauge
	diskQuota                    prometheus.Gauge
	fileCountExpected            *prometheus.GaugeVec
	fileCount                    *prometheus.GaugeVec
	fileAgeThreshold             *prometheus.GaugeVec
	fileYoungCount               *prometheus.GaugeVec
	latestFileCreationExpectedAt *prometheus.GaugeVec
	latestFileCreatedAt          *prometheus.GaugeVec
	latestFileCreationDuration   *prometheus.GaugeVec
	latestFileBornAt             *prometheus.GaugeVec
	latestFileModifiedAt         *prometheus.GaugeVec
	latestFileArchivedAt         *prometheus.GaugeVec
	latestSize                   *prometheus.GaugeVec
}

func NewDisk(diskName string) *DiskMetric {
	GetApplicationMetrics().disksTotal.Inc()

	presetLabels := map[string]string{"disk": diskName}
	disk := &DiskMetric{
		status: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "status",
			Help:        "Indicates whether there were any problems collecting metrics for this disk. Any value >0 means that errors occurred.",
			ConstLabels: presetLabels,
		}),
		fileCountExpected: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "files_maximum_count",
			Help:        "The amount of backup files expected to be present in this group.",
			ConstLabels: presetLabels,
		}, []string{
			LabelNameDir,
			LabelNameFile,
		}),
		fileCountTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Name:        "file_count_total",
			Help:        "The total amount of backup files present.",
			ConstLabels: presetLabels,
		}),
		fileCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "file_count",
			Help:        "The amount of backup files present in this group.",
			ConstLabels: presetLabels,
		}, []string{
			LabelNameDir,
			LabelNameFile,
			LabelNameGroup,
		}),
		diskUsageTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Name:        "disk_usage_bytes",
			Help:        "The amount of bytes used on a disk.",
			ConstLabels: presetLabels,
		}),
		diskQuota: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Name:        "disk_quota_bytes",
			Help:        "The amount of bytes used on a disk.",
			ConstLabels: presetLabels,
		}),
		fileAgeThreshold: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "files_maximum_age_seconds",
			Help:        "The maximum age (in seconds) that any file in this group should reach.",
			ConstLabels: presetLabels,
		}, []string{
			LabelNameDir,
			LabelNameFile,
		}),
		fileYoungCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "file_young_count",
			Help:        "The amount of backup files in this group that are younger than the maximum age (file_age_aim_seconds).",
			ConstLabels: presetLabels,
		}, []string{
			LabelNameDir,
			LabelNameFile,
			LabelNameGroup,
		}),
		latestFileCreationExpectedAt: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "latest_file_creation_expected_at",
			Help:        "Unix timestamp on which the latest backup in the corresponding file group should have occurred.",
			ConstLabels: presetLabels,
		}, []string{
			LabelNameDir,
			LabelNameFile,
		}),
		latestFileCreatedAt: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "latest_file_created_at",
			Help:        "Unix timestamp on which the latest backup in the corresponding file group was created.",
			ConstLabels: presetLabels,
		}, []string{
			LabelNameDir,
			LabelNameFile,
			LabelNameGroup,
		}),
		latestFileCreationDuration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "latest_file_creation_duration",
			Help:        "Describes how long it took to create the backup file in seconds",
			ConstLabels: presetLabels,
		}, []string{
			LabelNameDir,
			LabelNameFile,
			LabelNameGroup,
		}),
		latestFileBornAt: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "latest_file_born_at",
			Help:        "Unix timestamp on which the latest file has been initially created",
			ConstLabels: presetLabels,
		}, []string{
			LabelNameDir,
			LabelNameFile,
			LabelNameGroup,
		}),
		latestFileModifiedAt: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "latest_file_modified_at",
			Help:        "Unix timestamp on which the latest file has been modified",
			ConstLabels: presetLabels,
		}, []string{
			LabelNameDir,
			LabelNameFile,
			LabelNameGroup,
		}),
		latestFileArchivedAt: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "latest_file_archived_at",
			Help:        "Unix timestamp on which the latest file has been archived",
			ConstLabels: presetLabels,
		}, []string{
			LabelNameDir,
			LabelNameFile,
			LabelNameGroup,
		}),
		latestSize: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "latest_size_bytes",
			Help:        "Size (in bytes) of the latest backup in the corresponding file group.",
			ConstLabels: presetLabels,
		}, []string{
			LabelNameDir,
			LabelNameFile,
			LabelNameGroup,
		}),
	}
	registry.MustRegister(disk.status)
	registry.MustRegister(disk.fileCountTotal)
	registry.MustRegister(disk.diskUsageTotal)
	registry.MustRegister(disk.fileCountExpected)
	registry.MustRegister(disk.fileCount)
	registry.MustRegister(disk.fileAgeThreshold)
	registry.MustRegister(disk.fileYoungCount)
	registry.MustRegister(disk.latestFileCreationExpectedAt)
	registry.MustRegister(disk.latestFileCreatedAt)
	registry.MustRegister(disk.latestFileCreationDuration)
	registry.MustRegister(disk.latestFileBornAt)
	registry.MustRegister(disk.latestFileModifiedAt)
	registry.MustRegister(disk.latestFileArchivedAt)
	registry.MustRegister(disk.latestSize)
	return disk
}

func (b *DiskMetric) Drop() {
	registry.Unregister(b.status)
	registry.Unregister(b.fileCountTotal)
	registry.Unregister(b.diskUsageTotal)
	registry.Unregister(b.diskQuota)
	registry.Unregister(b.fileCountExpected)
	registry.Unregister(b.fileCount)
	registry.Unregister(b.fileAgeThreshold)
	registry.Unregister(b.fileYoungCount)
	registry.Unregister(b.latestFileCreationExpectedAt)
	registry.Unregister(b.latestFileCreatedAt)
	registry.Unregister(b.latestFileCreationDuration)
	registry.Unregister(b.latestFileBornAt)
	registry.Unregister(b.latestFileModifiedAt)
	registry.Unregister(b.latestFileArchivedAt)
	registry.Unregister(b.latestSize)

	GetApplicationMetrics().disksTotal.Dec()
}

func (b *DiskMetric) resetMetrics() {
	b.fileCountExpected.Reset()
	b.fileCount.Reset()
	b.fileAgeThreshold.Reset()
	b.fileYoungCount.Reset()
	b.latestFileCreationExpectedAt.Reset()
	b.latestFileCreatedAt.Reset()
	b.latestFileCreationDuration.Reset()
	b.latestFileBornAt.Reset()
	b.latestFileModifiedAt.Reset()
	b.latestFileArchivedAt.Reset()
	b.latestSize.Reset()
}

func (b *DiskMetric) DefinitionsMissing() {
	b.status.Set(1)
	registry.Unregister(b.diskQuota)
	b.resetMetrics()
}

func (b *DiskMetric) DefinitionsUpdated() {
	b.status.Set(0)
	b.resetMetrics()
}

func (b *DiskMetric) UpdateFileLimits(dir string, file string, count uint64, age time.Duration, ctime time.Time) {
	b.fileCountExpected.WithLabelValues(dir, file).Set(float64(count))
	b.fileAgeThreshold.WithLabelValues(dir, file).Set(age.Seconds())
	b.latestFileCreationExpectedAt.WithLabelValues(dir, file).Set(float64(ctime.Unix()))
}

func (b *DiskMetric) UpdateFileCounts(dir string, file string, group string, present int, young uint64) {
	b.fileCount.WithLabelValues(dir, file, group).Set(float64(present))
	b.fileYoungCount.WithLabelValues(dir, file, group).Set(float64(young))

	if present == 0 {
		labels := make(map[string]string)
		labels[LabelNameDir] = dir
		labels[LabelNameFile] = file
		labels[LabelNameGroup] = group

		b.deleteLatestFileLabels(labels)
	}
}

func (b *DiskMetric) UpdateUsageStats(countTotal uint64, sizeTotal uint64) {
	b.fileCountTotal.Set(float64(countTotal))
	b.diskUsageTotal.Set(float64(sizeTotal))
}

func (b *DiskMetric) UpdateDiskQuota(quota uint64) {
	if quota > 0 {
		err := registry.Register(b.diskQuota)
		if err != nil {
			if errors.Is(err, err.(prometheus.AlreadyRegisteredError)) {
				log.Debugf("Disk quote metric is already registered")
			} else {
				log.Errorf("Failed to register disk quota metric, %v", err)
			}
		}
		b.diskQuota.Set(float64(quota))
	} else {
		registry.Unregister(b.diskQuota)
	}
}

func (b *DiskMetric) deleteLatestFileLabels(labels map[string]string) {
	b.latestFileCreatedAt.Delete(labels)
	b.latestFileCreationDuration.Delete(labels)
	b.latestFileBornAt.Delete(labels)
	b.latestFileModifiedAt.Delete(labels)
	b.latestFileArchivedAt.Delete(labels)
	b.latestSize.Delete(labels)
}

func (b *DiskMetric) UpdateLatestFile(dir string, file string, group string, fileInfo *fs.FileInfo, time time.Time) {
	b.latestFileCreatedAt.WithLabelValues(dir, file, group).Set(float64(time.Unix()))
	b.latestFileCreationDuration.WithLabelValues(dir, file, group).Set(float64(fileInfo.ModifiedAt.Unix()) - float64(fileInfo.BornAt.Unix()))
	b.latestFileBornAt.WithLabelValues(dir, file, group).Set(float64(fileInfo.BornAt.Unix()))
	b.latestFileModifiedAt.WithLabelValues(dir, file, group).Set(float64(fileInfo.ModifiedAt.Unix()))
	b.latestFileArchivedAt.WithLabelValues(dir, file, group).Set(float64(fileInfo.ArchivedAt.Unix()))
	b.latestSize.WithLabelValues(dir, file, group).Set(float64(fileInfo.Size))
}

func (b *DiskMetric) DropFile(dir string, file string, group string) {
	labels := make(map[string]string)
	labels[LabelNameDir] = dir
	labels[LabelNameFile] = file
	labels[LabelNameGroup] = group

	b.fileCount.Delete(labels)
	b.fileYoungCount.Delete(labels)

	b.deleteLatestFileLabels(labels)
}
