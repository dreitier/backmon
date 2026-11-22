package storage

import (
	"testing"

	"github.com/dreitier/backmon/config"
	"github.com/dreitier/backmon/metrics"
)

// environments parses the `environments:` section of a configuration file
func environments(t *testing.T, yaml string) []*config.EnvironmentConfiguration {
	t.Helper()

	raw, err := config.ParseFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	return config.NewConfigurationInstance(raw).Environments()
}

func registerCleanup(t *testing.T) {
	t.Helper()

	withClients(t, map[string]*clientData{})

	// the metrics of a disk are registered globally, so they must not outlive the test
	registered := clients
	t.Cleanup(func() {
		for _, client := range registered {
			client.dropAllDisks()
		}
	})
}

func TestApplyConfiguration_registersAndDropsEnvironments(t *testing.T) {
	registerCleanup(t)

	ApplyConfiguration(environments(t, "environments:\n  first:\n    path: /mnt/first\n  second:\n    path: /mnt/second\n"))

	if len(clients) != 2 || clients["first"] == nil || clients["second"] == nil {
		t.Fatalf("got environments %v, want first and second", clients)
	}

	clients["second"].Disks["disk"] = &DiskData{Name: "disk", metrics: metrics.NewDisk("reload-dropped-disk")}

	ApplyConfiguration(environments(t, "environments:\n  first:\n    path: /mnt/first\n  third:\n    path: /mnt/third\n"))

	if len(clients) != 2 || clients["first"] == nil || clients["third"] == nil {
		t.Fatalf("got environments %v, want first and third", clients)
	}

	if len(GetDisks()) != 0 {
		t.Errorf("got disks %v, want the disk of the removed environment to be dropped", GetDisks())
	}
}

func TestApplyConfiguration_keepsDisksOfChangedEnvironment(t *testing.T) {
	registerCleanup(t)

	ApplyConfiguration(environments(t, "environments:\n  env:\n    s3:\n      access_key_id: old\n"))

	before := clients["env"]
	oldClient := before.Client
	disk := &DiskData{Name: "disk", metrics: metrics.NewDisk("reload-kept-disk")}
	before.Disks["disk"] = disk

	ApplyConfiguration(environments(t, "environments:\n  env:\n    definitions: other.yaml\n    s3:\n      access_key_id: rotated\n"))

	after := clients["env"]

	if after.Client == oldClient {
		t.Error("the client must be replaced, it holds the credentials")
	}

	if after.DefinitionFilename != "other.yaml" || after.Definition.Name != "other.yaml" {
		t.Errorf("definitions file = %q / %q, want other.yaml", after.DefinitionFilename, after.Definition.Name)
	}

	if after.Disks["disk"] != disk {
		t.Errorf("got disks %v, want the disk to be kept until the next update", after.Disks)
	}
}

func TestUpdateDiskInfo_dropsDiskExcludedByReloadedConfiguration(t *testing.T) {
	diskName := useLocalDisk(t, map[string]string{"backup_definitions.yaml": testDefinitions})

	UpdateDiskInfo()
	onlyDisk(t)

	raw, err := config.ParseFromString("all_others: exclude\n")
	if err != nil {
		t.Fatal(err)
	}

	clients["local"].disksFilter = config.ParseDisksSection(raw)

	UpdateDiskInfo()

	if disks := GetDisks(); len(disks) != 0 {
		t.Errorf("got %d disks, want %s to be dropped", len(disks), diskName)
	}
}
