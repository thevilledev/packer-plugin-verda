package instance

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

func TestBuilderRunRetainsSuccessfulInstanceUntilArtifactDestroy(t *testing.T) {
	client := builderTestClient()
	b := preparedTestBuilder(t, client, nil)

	artifactValue, err := b.Run(context.Background(), &packer.MockUi{}, &packer.MockHook{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	artifact := artifactValue.(*Artifact)
	if artifact.Id() != "inst-1" {
		t.Fatalf("artifact ID = %q", artifact.Id())
	}
	if len(client.deleteInstanceCallIDs) != 0 {
		t.Fatalf("instance deleted before artifact return: %#v", client.deleteInstanceCallIDs)
	}

	if err := artifact.Destroy(); err != nil {
		t.Fatalf("Destroy returned error: %v", err)
	}
	if len(client.deleteInstanceCallIDs) != 1 || client.deleteInstanceCallIDs[0] != "inst-1" {
		t.Fatalf("deleted instances = %#v", client.deleteInstanceCallIDs)
	}
}

func TestBuilderRunKeepInstanceDisablesArtifactDestroy(t *testing.T) {
	client := builderTestClient()
	b := preparedTestBuilder(t, client, map[string]interface{}{"keep_instance": true})

	artifact, err := b.Run(context.Background(), &packer.MockUi{}, &packer.MockHook{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if err := artifact.Destroy(); err != nil {
		t.Fatalf("Destroy returned error: %v", err)
	}
	if len(client.deleteInstanceCallIDs) != 0 {
		t.Fatalf("deleted instances = %#v", client.deleteInstanceCallIDs)
	}
}

func TestBuilderRunPromotesOSVolumeAndCleansBuildInstance(t *testing.T) {
	client := &fakeClient{
		cloneVolumeIDs: []string{"vol-artifact"},
		instancesByID: map[string][]*verda.Instance{
			"inst-1": {fakeInstance("inst-1"), {
				ID: "inst-1", Status: verda.StatusOffline, Location: "FIN-03",
				OSVolumeID: stringPointer("vol-os"),
			}},
		},
		volumesByID: map[string][]*verda.Volume{
			"vol-os": {{
				ID: "vol-os", Name: "source", Type: verda.VolumeTypeNVMe,
				Status: verda.VolumeStatusAttached, Location: "FIN-03",
			}},
			"vol-artifact": {{
				ID: "vol-artifact", Name: "artifact", Type: verda.VolumeTypeNVMe,
				Status: verda.VolumeStatusDetached, Location: "FIN-03",
			}},
		},
	}
	b := preparedTestBuilder(t, client, map[string]interface{}{
		"artifact_type":                  artifactTypeOSVolume,
		"artifact_volume_location_codes": []string{"FIN-03"},
	})

	artifact, err := b.Run(context.Background(), &packer.MockUi{}, &packer.MockHook{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if artifact.Id() != "vol-artifact" {
		t.Fatalf("artifact ID = %q", artifact.Id())
	}
	if len(client.deleteInstanceCallIDs) != 1 || client.deleteInstanceCallIDs[0] != "inst-1" {
		t.Fatalf("deleted build instances = %#v", client.deleteInstanceCallIDs)
	}
	if len(client.deleteVolumeCallIDs) != 0 {
		t.Fatalf("artifact volume deleted before return: %#v", client.deleteVolumeCallIDs)
	}
	if err := artifact.Destroy(); err != nil {
		t.Fatalf("Destroy returned error: %v", err)
	}
	if len(client.deleteVolumeCallIDs) != 1 || client.deleteVolumeCallIDs[0] != "vol-artifact" {
		t.Fatalf("deleted artifact volumes = %#v", client.deleteVolumeCallIDs)
	}
}

func TestBuilderRunCancellationCleansIncompleteInstance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := builderTestClient()
	client.createInstanceFn = func(context.Context, verda.CreateInstanceRequest) (*verda.Instance, error) {
		cancel()
		return fakeInstance("inst-cancelled"), nil
	}
	b := preparedTestBuilder(t, client, nil)

	artifact, err := b.Run(ctx, &packer.MockUi{}, &packer.MockHook{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
	if artifact != nil {
		t.Fatalf("artifact = %#v, want nil", artifact)
	}
	if len(client.deleteInstanceCallIDs) != 1 || client.deleteInstanceCallIDs[0] != "inst-cancelled" {
		t.Fatalf("deleted instances = %#v", client.deleteInstanceCallIDs)
	}
}

func TestBuilderRunCancellationHonorsKeepInstance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := builderTestClient()
	client.createInstanceFn = func(context.Context, verda.CreateInstanceRequest) (*verda.Instance, error) {
		cancel()
		return fakeInstance("inst-retained"), nil
	}
	b := preparedTestBuilder(t, client, map[string]interface{}{"keep_instance": true})

	if _, err := b.Run(ctx, &packer.MockUi{}, &packer.MockHook{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
	if len(client.deleteInstanceCallIDs) != 0 {
		t.Fatalf("deleted instances = %#v", client.deleteInstanceCallIDs)
	}
}

func TestBuilderCanRunTwiceWithoutReusingTemporaryResourceIDs(t *testing.T) {
	client := builderTestClient()
	client.createSSHKeyIDs = []string{"key-first", "key-second"}
	b := preparedTestBuilder(t, client, map[string]interface{}{
		"ssh_key_ids": []string{"key-existing"},
	})

	for range 2 {
		artifact, err := b.Run(context.Background(), &packer.MockUi{}, &packer.MockHook{})
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		if err := artifact.Destroy(); err != nil {
			t.Fatalf("Destroy returned error: %v", err)
		}
	}

	want := [][]string{{"key-first", "key-existing"}, {"key-second", "key-existing"}}
	if len(client.createInstanceReqs) != len(want) {
		t.Fatalf("create instance requests = %d", len(client.createInstanceReqs))
	}
	for i, req := range client.createInstanceReqs {
		if len(req.SSHKeyIDs) != 2 || req.SSHKeyIDs[0] != want[i][0] || req.SSHKeyIDs[1] != want[i][1] {
			t.Fatalf("request %d SSHKeyIDs = %#v, want %#v", i, req.SSHKeyIDs, want[i])
		}
	}
	if len(b.config.SSHKeyIDs) != 1 || b.config.SSHKeyIDs[0] != "key-existing" {
		t.Fatalf("prepared config SSHKeyIDs mutated to %#v", b.config.SSHKeyIDs)
	}
}

func preparedTestBuilder(t *testing.T, client *fakeClient, overrides map[string]interface{}) *Builder {
	t.Helper()
	config := map[string]interface{}{
		"client_id":               "client-id",
		"client_secret":           "client-secret",
		"instance_type":           "V100",
		"image":                   "ubuntu-24.04",
		"hostname":                "packer-test",
		"communicator":            "none",
		"temporary_key_pair_type": "ed25519",
	}
	for key, value := range overrides {
		config[key] = value
	}
	b := &Builder{clientFactory: func(*Config) (verdaClient, error) { return client, nil }}
	if _, _, err := b.Prepare(config); err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	return b
}

func builderTestClient() *fakeClient {
	return &fakeClient{instancesByID: map[string][]*verda.Instance{
		"inst-1": {fakeInstance("inst-1")},
	}}
}

func fakeInstance(id string) *verda.Instance {
	ip := "203.0.113.10"
	osVolumeID := "vol-os"
	return &verda.Instance{
		ID:           id,
		IP:           &ip,
		Status:       verda.StatusRunning,
		Location:     "FIN-03",
		InstanceType: "V100",
		OSVolumeID:   &osVolumeID,
	}
}

func stringPointer(value string) *string {
	return &value
}
