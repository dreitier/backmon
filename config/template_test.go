package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// config.yaml.template has to stay a valid configuration with one example for each kind of storage
func Test_configTemplateIsValid(t *testing.T) {
	file, err := os.Open("../config.yaml.template")
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	raw, err := Parse(file)
	require.NoError(t, err)

	sut := NewConfigurationInstance(raw)

	environments := make(map[string]*ClientConfiguration)
	for _, env := range sut.Environments() {
		environments[env.Name] = env.Client
	}

	require.Len(t, environments, 4, "an environment of the template could not be parsed")

	aws := environments["aws-test-environment"]
	require.NotNil(t, aws)
	assert.Equal(t, "AWS_ACCESS_KEY_1", aws.AccessKey)
	assert.Equal(t, "eu-central-1", aws.Region)
	assert.True(t, aws.Disks.IsDiskIncluded("my-bucket-1"))
	assert.False(t, aws.Disks.IsDiskIncluded("my-secret-bucket-2"))

	minio := environments["local-minio-environment"]
	require.NotNil(t, minio)
	assert.Equal(t, "http://my-minio-endpoint:9000", minio.Endpoint)
	assert.True(t, minio.ForcePathStyle)
	assert.Equal(t, "http://my-proxy:3128", minio.Proxy)

	gcs := environments["gcs-environment"]
	require.NotNil(t, gcs)
	require.NotNil(t, gcs.GCS)
	assert.Equal(t, "my-gcp-project", gcs.GCS.ProjectID)
	assert.Equal(t, "/etc/backmon/gcs-service-account.json", gcs.GCS.CredentialsFile)
	assert.True(t, gcs.AutoDiscoverDisks)
	assert.False(t, gcs.Disks.IsDiskIncluded("my-secret-bucket-3"))

	local := environments["local-environment"]
	require.NotNil(t, local)
	assert.Equal(t, "/mnt/backup", local.Directory)
}
