package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

type Bucket struct {
	status              prometheus.Gauge
	fileCountExpected   *prometheus.GaugeVec
	fileCount           *prometheus.GaugeVec
	fileAgeThreshold    *prometheus.GaugeVec
	fileYoungCount      *prometheus.GaugeVec
	latestCtimeExpected *prometheus.GaugeVec
	latestCtime         *prometheus.GaugeVec
	latestSize          *prometheus.GaugeVec
}

func NewBucket(bucketName string) *Bucket {
	presetLabels := map[string]string{"bucket": bucketName}
	bucket := &Bucket{
		status: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "status",
			Help:      "Indicates whether there were any problems collecting metrics for this bucket. Any value >0 means that errors occurred.",
			ConstLabels: presetLabels,
		}),
		fileCountExpected: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "file_count_aim",
			Help:      "The amount of backup files expected to be present in this group.",
			ConstLabels: presetLabels,
		}, []string{
			"dir",
			"file",
		}),
		fileCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "file_count",
			Help:      "The amount of backup files present in this group.",
			ConstLabels: presetLabels,
		}, []string{
			"dir",
			"file",
			"group",
		}),
		fileAgeThreshold: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "file_age_aim_seconds",
			Help:      "The maximum age (in seconds) that any file in this group should reach.",
			ConstLabels: presetLabels,
		}, []string{
			"dir",
			"file",
		}),
		fileYoungCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "file_young_count",
			Help:      "The amount of backup files in this group that are younger than the maximum age (file_age_aim_seconds).",
			ConstLabels: presetLabels,
		}, []string{
			"dir",
			"file",
			"group",
		}),
		latestCtimeExpected: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "latest_creation_aim_seconds",
			Help:      "Unix Time on which the latest backup in the corresponding file group should have occurred.",
			ConstLabels: presetLabels,
		}, []string{
			"dir",
			"file",
		}),
		latestCtime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "latest_creation_seconds",
			Help:      "Unix Time on which the latest backup in the corresponding file group was created.",
			ConstLabels: presetLabels,
		}, []string{
			"dir",
			"file",
			"group",
		}),
		latestSize: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "latest_size_bytes",
			Help:      "Size (in bytes) of the latest backup in the corresponding file group.",
			ConstLabels: presetLabels,
		}, []string{
			"dir",
			"file",
			"group",
		}),
	}
	registry.MustRegister(bucket.status)
	registry.MustRegister(bucket.fileCountExpected)
	registry.MustRegister(bucket.fileCount)
	registry.MustRegister(bucket.fileAgeThreshold)
	registry.MustRegister(bucket.fileYoungCount)
	registry.MustRegister(bucket.latestCtimeExpected)
	registry.MustRegister(bucket.latestCtime)
	registry.MustRegister(bucket.latestSize)
	return bucket
}

func (b *Bucket) Drop() {
	registry.Unregister(b.status)
	registry.Unregister(b.fileCountExpected)
	registry.Unregister(b.fileCount)
	registry.Unregister(b.fileAgeThreshold)
	registry.Unregister(b.fileYoungCount)
	registry.Unregister(b.latestCtimeExpected)
	registry.Unregister(b.latestCtime)
	registry.Unregister(b.latestSize)
}

func (b *Bucket) DefinitionsMissing() {
	b.status.Set(1)
	b.fileCountExpected.Reset()
	b.fileCount.Reset()
	b.fileAgeThreshold.Reset()
	b.fileYoungCount.Reset()
	b.latestCtimeExpected.Reset()
	b.latestCtime.Reset()
	b.latestSize.Reset()
}

func (b *Bucket) DefinitionsUpdated() {
	b.status.Set(0)
	b.fileCountExpected.Reset()
	b.fileCount.Reset()
	b.fileAgeThreshold.Reset()
	b.fileYoungCount.Reset()
	b.latestCtimeExpected.Reset()
	b.latestCtime.Reset()
	b.latestSize.Reset()
}

func (b *Bucket) FileLimits(dir string, file string, count uint64, age time.Duration, ctime time.Time) {
	b.fileCountExpected.WithLabelValues(dir, file).Set(float64(count))
	b.fileAgeThreshold.WithLabelValues(dir, file).Set(age.Seconds())
	b.latestCtimeExpected.WithLabelValues(dir, file).Set(float64(ctime.Unix()))
}

func (b *Bucket) FileCounts(dir string, file string, group string, present int, young uint64) {
	b.fileCount.WithLabelValues(dir, file, group).Set(float64(present))
	b.fileYoungCount.WithLabelValues(dir, file, group).Set(float64(young))
	if present == 0 {
		labels := make(map[string]string)
		labels["dir"] = dir
		labels["file"] = file
		labels["group"] = group
		b.latestCtime.Delete(labels)
		b.latestSize.Delete(labels)
	}
}

func (b *Bucket) LatestFile(dir string, file string, group string, size int64, time time.Time) {
	b.latestCtime.WithLabelValues(dir, file, group).Set(float64(time.Unix()))
	b.latestSize.WithLabelValues(dir, file, group).Set(float64(size))
}

func (b *Bucket) DropFile(dir string, file string, group string) {
	labels := make(map[string]string)
	labels["dir"] = dir
	labels["file"] = file
	labels["group"] = group
	b.fileCount.Delete(labels)
	b.fileYoungCount.Delete(labels)
	b.latestCtime.Delete(labels)
	b.latestSize.Delete(labels)
}
