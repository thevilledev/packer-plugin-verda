package instance

import "testing"

func TestArtifactDestroyDeletesVolumeReplicas(t *testing.T) {
	client := &fakeClient{}
	artifact := &Artifact{
		Client:       client,
		ArtifactType: artifactTypeOSVolume,
		VolumeID:     "vol-fin-01",
		DeleteConfig: DeleteConfig{
			VolumeIDs:       []string{"vol-fin-01", "vol-fin-03"},
			DeleteOnDestroy: true,
		},
	}

	if err := artifact.Destroy(); err != nil {
		t.Fatalf("Destroy returned error: %s", err)
	}
	if len(client.deleteVolumeCallIDs) != 2 ||
		client.deleteVolumeCallIDs[0] != "vol-fin-01" ||
		client.deleteVolumeCallIDs[1] != "vol-fin-03" {
		t.Fatalf("deleted volumes = %#v", client.deleteVolumeCallIDs)
	}
}
