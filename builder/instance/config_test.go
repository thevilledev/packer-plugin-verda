package instance

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/hcl/v2/hclparse"
)

func TestBuilderConfigSpecIncludesCommunicatorFields(t *testing.T) {
	var b Builder
	spec := b.ConfigSpec()

	communicatorFields := []string{
		"communicator",
		"ssh_username",
		"ssh_private_key_file",
		"temporary_key_pair_name",
		"temporary_key_pair_type",
		"temporary_key_pair_bits",
	}
	for _, field := range communicatorFields {
		if _, ok := spec[field]; !ok {
			t.Fatalf("ConfigSpec() missing communicator field %q", field)
		}
	}

	if _, ok := spec["ssh_username"].(*hcldec.AttrSpec); !ok {
		t.Fatalf("ssh_username spec = %T, want *hcldec.AttrSpec", spec["ssh_username"])
	}
}

func TestConfigPrepareHCLCommunicatorFields(t *testing.T) {
	var b Builder
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL([]byte(`
client_id               = "client-id"
client_secret           = "client-secret"
instance_type           = "V100"
image                   = "ubuntu-24.04"
hostname                = "packer-test"
ssh_username            = "ubuntu"
temporary_key_pair_name = "packer-test"
`), "test.pkr.hcl")
	if diags.HasErrors() {
		t.Fatalf("ParseHCL returned errors: %s", diags.Error())
	}

	val, diags := hcldec.Decode(file.Body, b.ConfigSpec(), nil)
	if diags.HasErrors() {
		t.Fatalf("Decode returned errors: %s", diags.Error())
	}

	var c Config
	if _, _, err := c.Prepare(val); err != nil {
		t.Fatalf("Prepare returned error: %s", err)
	}
	if c.Comm.SSHUsername != "ubuntu" {
		t.Fatalf("SSHUsername = %q", c.Comm.SSHUsername)
	}
	if c.Comm.SSHTemporaryKeyPairName != "packer-test" {
		t.Fatalf("SSHTemporaryKeyPairName = %q", c.Comm.SSHTemporaryKeyPairName)
	}
}

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

func TestConfigPrepareArtifactVolumeLocationCodes(t *testing.T) {
	var c Config
	_, _, err := c.Prepare(map[string]interface{}{
		"client_id":                      "client-id",
		"client_secret":                  "client-secret",
		"instance_type":                  "V100",
		"image":                          "ubuntu-24.04",
		"hostname":                       "packer-test",
		"artifact_type":                  "os_volume",
		"artifact_volume_location_codes": []string{"FIN-01", "FIN-03"},
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %s", err)
	}
	if c.ArtifactVolumeLocationCode != "FIN-01" {
		t.Fatalf("ArtifactVolumeLocationCode = %q", c.ArtifactVolumeLocationCode)
	}
	if len(c.ArtifactVolumeLocationCodes) != 2 || c.ArtifactVolumeLocationCodes[1] != "FIN-03" {
		t.Fatalf("ArtifactVolumeLocationCodes = %#v", c.ArtifactVolumeLocationCodes)
	}
}

func TestConfigPrepareArtifactVolumeLocationCodesRejectsDuplicates(t *testing.T) {
	var c Config
	_, _, err := c.Prepare(map[string]interface{}{
		"client_id":                      "client-id",
		"client_secret":                  "client-secret",
		"instance_type":                  "V100",
		"image":                          "ubuntu-24.04",
		"hostname":                       "packer-test",
		"artifact_type":                  "os_volume",
		"artifact_volume_location_codes": []string{"FIN-01", "FIN-01"},
	})
	if err == nil {
		t.Fatal("expected duplicate artifact volume locations to fail")
	}
}

func TestConfigPrepareArtifactVolumeLocationCodesRequireCloning(t *testing.T) {
	clone := false
	var c Config
	_, _, err := c.Prepare(map[string]interface{}{
		"client_id":                      "client-id",
		"client_secret":                  "client-secret",
		"instance_type":                  "V100",
		"image":                          "ubuntu-24.04",
		"hostname":                       "packer-test",
		"artifact_type":                  "os_volume",
		"clone_os_volume":                clone,
		"artifact_volume_location_codes": []string{"FIN-01", "FIN-03"},
	})
	if err == nil {
		t.Fatal("expected multi-location artifact without cloning to fail")
	}
}
