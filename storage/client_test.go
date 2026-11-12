package storage

import (
	"testing"

	"github.com/dreitier/backmon/config"
	"github.com/dreitier/backmon/storage/provider"
)

// When a Directory is configured, NewClient must fall back to the local
// filesystem provider and carry the environment name and directory through.
func TestNewClientReturnsLocalClientWhenDirectorySet(t *testing.T) {
	cfg := &config.ClientConfiguration{
		EnvName:   "local-env",
		Directory: "/mnt/backup",
	}

	client := NewClient(cfg)

	local, ok := client.(*provider.LocalClient)
	if !ok {
		t.Fatalf("expected *provider.LocalClient, got %T", client)
	}
	if local.Directory != "/mnt/backup" {
		t.Errorf("Directory = %q, want %q", local.Directory, "/mnt/backup")
	}
	if local.EnvName != "local-env" {
		t.Errorf("EnvName = %q, want %q", local.EnvName, "local-env")
	}
}

// Without a Directory, NewClient must produce an S3 provider and pass the
// S3-specific configuration fields through unchanged.
func TestNewClientReturnsS3ClientWhenDirectoryEmpty(t *testing.T) {
	cfg := &config.ClientConfiguration{
		EnvName:           "s3-env",
		Region:            "eu-central-1",
		AccessKey:         "AKIA",
		SecretKey:         "secret",
		AssumeRoleArn:     "arn:aws:iam::123:role/r",
		Endpoint:          "https://s3.example.com",
		TLSSkipVerify:     true,
		ForcePathStyle:    true,
		Token:             "token",
		AutoDiscoverDisks: true,
	}

	client := NewClient(cfg)

	s3c, ok := client.(*provider.S3Client)
	if !ok {
		t.Fatalf("expected *provider.S3Client, got %T", client)
	}

	if s3c.EnvName != "s3-env" {
		t.Errorf("EnvName = %q, want %q", s3c.EnvName, "s3-env")
	}
	if s3c.Region != "eu-central-1" {
		t.Errorf("Region = %q, want %q", s3c.Region, "eu-central-1")
	}
	if s3c.AccessKey != "AKIA" || s3c.SecretKey != "secret" {
		t.Errorf("credentials not propagated: %q / %q", s3c.AccessKey, s3c.SecretKey)
	}
	if s3c.AssumeRoleArn != "arn:aws:iam::123:role/r" {
		t.Errorf("AssumeRoleArn = %q", s3c.AssumeRoleArn)
	}
	if s3c.Endpoint != "https://s3.example.com" {
		t.Errorf("Endpoint = %q", s3c.Endpoint)
	}
	if !s3c.TLSSkipVerify || !s3c.ForcePathStyle || !s3c.AutoDiscoverDisks {
		t.Errorf("boolean flags not propagated: %+v", s3c)
	}
	if s3c.Token != "token" {
		t.Errorf("Token = %q", s3c.Token)
	}
}
