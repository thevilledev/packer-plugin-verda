package instance

import (
	"errors"
	"strings"
	"testing"
)

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

func TestArtifactDestroyDeletesInstance(t *testing.T) {
	client := &fakeClient{}
	artifact := &Artifact{
		Client:       client,
		InstanceID:   "inst-1",
		ArtifactType: artifactTypeInstance,
		DeleteConfig: DeleteConfig{DeleteOnDestroy: true},
	}
	if err := artifact.Destroy(); err != nil {
		t.Fatalf("Destroy returned error: %v", err)
	}
	if len(client.deleteInstanceCallIDs) != 1 || client.deleteInstanceCallIDs[0] != "inst-1" {
		t.Fatalf("deleted instances = %#v", client.deleteInstanceCallIDs)
	}
}

func TestArtifactDestroyHonorsRetention(t *testing.T) {
	client := &fakeClient{}
	artifact := &Artifact{
		Client:       client,
		InstanceID:   "inst-1",
		ArtifactType: artifactTypeInstance,
	}
	if err := artifact.Destroy(); err != nil {
		t.Fatalf("Destroy returned error: %v", err)
	}
	if len(client.deleteInstanceCallIDs) != 0 {
		t.Fatalf("deleted instances = %#v", client.deleteInstanceCallIDs)
	}
}

func TestArtifactDestroyJoinsVolumeErrors(t *testing.T) {
	errFirst := errors.New("first delete failed")
	errSecond := errors.New("second delete failed")
	client := &fakeClient{deleteVolumeErrs: map[string]error{
		"vol-1": errFirst,
		"vol-2": errSecond,
	}}
	artifact := &Artifact{
		Client:       client,
		ArtifactType: artifactTypeOSVolume,
		DeleteConfig: DeleteConfig{
			VolumeIDs:       []string{"vol-1", "vol-2"},
			DeleteOnDestroy: true,
		},
	}
	err := artifact.Destroy()
	if !errors.Is(err, errFirst) || !errors.Is(err, errSecond) {
		t.Fatalf("Destroy error = %v", err)
	}
}

func TestArtifactInterfaceMetadata(t *testing.T) {
	artifact := &Artifact{
		ArtifactType: artifactTypeInstance,
		InstanceID:   "inst-1",
		InstanceIP:   "203.0.113.10",
		StateData:    map[string]interface{}{"Location": "FIN-03"},
	}
	if artifact.BuilderId() != BuilderID {
		t.Fatalf("BuilderId = %q", artifact.BuilderId())
	}
	if artifact.Files() != nil {
		t.Fatalf("Files = %#v", artifact.Files())
	}
	if artifact.Id() != "inst-1" {
		t.Fatalf("Id = %q", artifact.Id())
	}
	if !strings.Contains(artifact.String(), "inst-1") {
		t.Fatalf("String = %q", artifact.String())
	}
	if artifact.State("Location") != "FIN-03" {
		t.Fatalf("Location state = %#v", artifact.State("Location"))
	}
}

func TestArtifactStringForOSVolumes(t *testing.T) {
	t.Run("single volume includes location", func(t *testing.T) {
		artifact := &Artifact{
			ArtifactType:     artifactTypeOSVolume,
			VolumeID:         "vol-fin-03",
			VolumeIDs:        []string{"vol-fin-03"},
			VolumeLocation:   "FIN-03",
			VolumeLocations:  []string{"FIN-03"},
			SourceOSVolumeID: "vol-source",
			ClonedVolume:     true,
		}
		want := "Verda OS volume: vol-fin-03 (FIN-03) (cloned from vol-source)"
		if got := artifact.String(); got != want {
			t.Fatalf("String() = %q, want %q", got, want)
		}
	})

	t.Run("multiple volumes include every ordered location", func(t *testing.T) {
		artifact := &Artifact{
			ArtifactType:     artifactTypeOSVolume,
			VolumeID:         "vol-fin-02",
			VolumeIDs:        []string{"vol-fin-02", "vol-fin-03"},
			VolumeLocations:  []string{"FIN-02", "FIN-03"},
			SourceOSVolumeID: "vol-source",
			ClonedVolume:     true,
		}
		want := "Verda OS volumes (2):\n" +
			"  FIN-02: vol-fin-02 (primary)\n" +
			"  FIN-03: vol-fin-03\n" +
			"Cloned from: vol-source"
		if got := artifact.String(); got != want {
			t.Fatalf("String() = %q, want %q", got, want)
		}
	})
}
