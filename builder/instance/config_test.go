package instance

import (
	"testing"
)

func TestConfigPrepareDefaults(t *testing.T) {
	t.Setenv("VERDA_CLIENT_ID", "client-id")
	t.Setenv("VERDA_CLIENT_SECRET", "client-secret")

	var c Config
	_, _, err := c.Prepare(map[string]interface{}{
		"instance_type": "V100",
		"image":         "ubuntu-24.04",
		"hostname":      "packer-test",
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %s", err)
	}

	if c.ClientID != "client-id" {
		t.Fatalf("ClientID = %q", c.ClientID)
	}
	if c.ClientSecret != "client-secret" {
		t.Fatalf("ClientSecret = %q", c.ClientSecret)
	}
	if c.LocationCode != defaultLocationCode {
		t.Fatalf("LocationCode = %q", c.LocationCode)
	}
	if c.Comm.Type != "ssh" {
		t.Fatalf("Comm.Type = %q", c.Comm.Type)
	}
	if c.Comm.SSHUsername != defaultSSHUsername {
		t.Fatalf("SSHUsername = %q", c.Comm.SSHUsername)
	}
	if c.Description == "" {
		t.Fatal("Description was not defaulted")
	}
	if c.ArtifactType != artifactTypeInstance {
		t.Fatalf("ArtifactType = %q", c.ArtifactType)
	}
	if !c.shouldCloneOSVolume() {
		t.Fatal("expected OS volume clone to default true")
	}
}

func TestConfigPrepareValidation(t *testing.T) {
	var c Config
	_, _, err := c.Prepare(map[string]interface{}{
		"client_id":     "client-id",
		"client_secret": "client-secret",
		"hostname":      "packer-test",
		"image":         "ubuntu-24.04",
	})
	if err == nil {
		t.Fatal("expected missing instance_type to fail")
	}
}

func TestStatusAllowed(t *testing.T) {
	if !statusAllowed("RUNNING", []string{"running"}) {
		t.Fatal("expected case-insensitive status match")
	}
	if statusAllowed("provisioning", []string{"running"}) {
		t.Fatal("unexpected status match")
	}
}

func TestConfigPrepareOSVolumeArtifactValidation(t *testing.T) {
	clone := false
	var c Config
	_, _, err := c.Prepare(map[string]interface{}{
		"client_id":       "client-id",
		"client_secret":   "client-secret",
		"instance_type":   "V100",
		"image":           "ubuntu-24.04",
		"hostname":        "packer-test",
		"artifact_type":   "os_volume",
		"clone_os_volume": clone,
		"keep_instance":   true,
	})
	if err == nil {
		t.Fatal("expected keep_instance with un-cloned OS volume artifact to fail")
	}
}
