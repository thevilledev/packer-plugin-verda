package instance

import (
	"strings"
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
	if c.Comm.SSHTemporaryKeyPairName != c.TemporarySSHKeyName {
		t.Fatalf("SSHTemporaryKeyPairName = %q, TemporarySSHKeyName = %q", c.Comm.SSHTemporaryKeyPairName, c.TemporarySSHKeyName)
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

func TestConfigPrepareRejectsInvalidInstanceRequestValues(t *testing.T) {
	tests := []struct {
		name     string
		override map[string]interface{}
		contains string
	}{
		{name: "contract", override: map[string]interface{}{"contract": "FREE"}, contains: "contract"},
		{name: "OS volume name without size", override: map[string]interface{}{"os_volume_name": "custom-os"}, contains: "os_volume"},
		{name: "negative OS volume size", override: map[string]interface{}{"os_volume_size": -1}, contains: "os_volume"},
		{name: "OS volume spot policy", override: map[string]interface{}{
			"os_volume_size": 10, "os_volume_spot_behavior": "explode",
		}, contains: "os_volume"},
		{name: "data volume type", override: map[string]interface{}{
			"volume": []map[string]interface{}{{"name": "data", "size": 10, "type": "magic"}},
		}, contains: "volumes"},
		{name: "data volume spot policy", override: map[string]interface{}{
			"volume": []map[string]interface{}{{
				"name": "data", "size": 10, "type": "NVMe", "on_spot_discontinue": "explode",
			}},
		}, contains: "on_spot_discontinue"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := validRawConfig()
			for key, value := range tt.override {
				config[key] = value
			}
			var c Config
			_, _, err := c.Prepare(config)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("error = %q, want substring %q", err, tt.contains)
			}
		})
	}
}

func TestConfigPrepareRejectsAmbiguousAndCaseDuplicateArtifactLocations(t *testing.T) {
	tests := []map[string]interface{}{
		{
			"artifact_volume_location_code":  "FIN-01",
			"artifact_volume_location_codes": []string{"FIN-03"},
		},
		{
			"artifact_volume_location_codes": []string{"FIN-01", "fin-01"},
		},
	}
	for _, override := range tests {
		config := validRawConfig()
		for key, value := range override {
			config[key] = value
		}
		var c Config
		if _, _, err := c.Prepare(config); err == nil {
			t.Fatalf("expected locations %#v to fail", override)
		}
	}
}

func TestConfigPrepareValidatesAllowedSSHStatuses(t *testing.T) {
	tests := [][]string{
		{"running", "RUNNING"},
		{"running", ""},
		{"error"},
	}
	for _, statuses := range tests {
		config := validRawConfig()
		config["allowed_ssh_statuses"] = statuses
		var c Config
		if _, _, err := c.Prepare(config); err == nil {
			t.Fatalf("expected statuses %#v to fail", statuses)
		}
	}
}

func TestConfigPrepareAcceptsValidVolumeConfiguration(t *testing.T) {
	config := validRawConfig()
	config["os_volume_size"] = 100
	config["os_volume_spot_behavior"] = "keep_detached"
	config["volume"] = []map[string]interface{}{{
		"name": "data", "size": 10, "type": "NVMe", "on_spot_discontinue": "move_to_trash",
	}}
	var c Config
	if _, _, err := c.Prepare(config); err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
}

func validRawConfig() map[string]interface{} {
	return map[string]interface{}{
		"client_id":     "client-id",
		"client_secret": "client-secret",
		"instance_type": "V100",
		"image":         "ubuntu-24.04",
		"hostname":      "packer-test",
	}
}
