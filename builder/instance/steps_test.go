package instance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

type fakeClient struct {
	createSSHKeyReq      *verda.CreateSSHKeyRequest
	deleteSSHKeyID       string
	createScriptReq      *verda.CreateStartupScriptRequest
	deleteScriptID       string
	createInstanceReq    *verda.CreateInstanceRequest
	deleteInstanceID     string
	deleteVolumeIDs      []string
	deletePermanently    bool
	shutdownInstanceID   string
	getVolumeCallCount   int
	cloneVolumeSourceID  string
	cloneVolumeSourceIDs []string
	cloneVolumeReq       *volumeCloneRequest
	cloneVolumeReqs      []volumeCloneRequest
	cloneVolumeIDs       []string
	cloneVolumeErrs      []error
	deleteVolumeID       string
	deleteVolumeForce    bool
	deleteVolumeCallIDs  []string
	instances            []*verda.Instance
	volumes              []*verda.Volume
	getInstanceCallCount int
	createInstanceErr    error
}

func (f *fakeClient) GetInstance(context.Context, string) (*verda.Instance, error) {
	if len(f.instances) == 0 {
		return nil, errors.New("no fake instances configured")
	}
	idx := f.getInstanceCallCount
	if idx >= len(f.instances) {
		idx = len(f.instances) - 1
	}
	f.getInstanceCallCount++
	return f.instances[idx], nil
}

func (f *fakeClient) CreateInstance(_ context.Context, req verda.CreateInstanceRequest) (*verda.Instance, error) {
	f.createInstanceReq = &req
	if f.createInstanceErr != nil {
		return nil, f.createInstanceErr
	}
	ip := "203.0.113.10"
	osVolumeID := "vol-os"
	return &verda.Instance{
		ID:           "inst-1",
		IP:           &ip,
		Status:       "running",
		InstanceType: req.InstanceType,
		Location:     req.LocationCode,
		OSVolumeID:   &osVolumeID,
	}, nil
}

func (f *fakeClient) DeleteInstance(_ context.Context, id string, volumeIDs []string, deletePermanently bool) error {
	f.deleteInstanceID = id
	f.deleteVolumeIDs = volumeIDs
	f.deletePermanently = deletePermanently
	return nil
}

func (f *fakeClient) ShutdownInstance(_ context.Context, id string) error {
	f.shutdownInstanceID = id
	return nil
}

func (f *fakeClient) GetVolume(_ context.Context, id string) (*verda.Volume, error) {
	if len(f.volumes) == 0 {
		return &verda.Volume{
			ID:       id,
			Name:     "volume-" + id,
			Status:   verda.VolumeStatusDetached,
			Location: "FIN-03",
		}, nil
	}
	idx := f.getVolumeCallCount
	if idx >= len(f.volumes) {
		idx = len(f.volumes) - 1
	}
	f.getVolumeCallCount++
	return f.volumes[idx], nil
}

func (f *fakeClient) CloneVolume(_ context.Context, id string, req volumeCloneRequest) (string, error) {
	f.cloneVolumeSourceID = id
	f.cloneVolumeSourceIDs = append(f.cloneVolumeSourceIDs, id)
	f.cloneVolumeReq = &req
	f.cloneVolumeReqs = append(f.cloneVolumeReqs, req)
	idx := len(f.cloneVolumeReqs) - 1
	if len(f.cloneVolumeErrs) > 0 {
		if idx >= len(f.cloneVolumeErrs) {
			idx = len(f.cloneVolumeErrs) - 1
		}
		if err := f.cloneVolumeErrs[idx]; err != nil {
			return "", err
		}
	}
	if len(f.cloneVolumeIDs) > 0 {
		if idx >= len(f.cloneVolumeIDs) {
			idx = len(f.cloneVolumeIDs) - 1
		}
		return f.cloneVolumeIDs[idx], nil
	}
	return "vol-cloned", nil
}

func (f *fakeClient) DeleteVolume(_ context.Context, id string, force bool) error {
	f.deleteVolumeID = id
	f.deleteVolumeForce = force
	f.deleteVolumeCallIDs = append(f.deleteVolumeCallIDs, id)
	return nil
}

func (f *fakeClient) CreateSSHKey(_ context.Context, req verda.CreateSSHKeyRequest) (*verda.SSHKey, error) {
	f.createSSHKeyReq = &req
	return &verda.SSHKey{ID: "key-1", Name: req.Name}, nil
}

func (f *fakeClient) DeleteSSHKey(_ context.Context, id string) error {
	f.deleteSSHKeyID = id
	return nil
}

func (f *fakeClient) CreateStartupScript(_ context.Context, req verda.CreateStartupScriptRequest) (*verda.StartupScript, error) {
	f.createScriptReq = &req
	return &verda.StartupScript{ID: "script-1", Name: req.Name}, nil
}

func (f *fakeClient) DeleteStartupScript(_ context.Context, id string) error {
	f.deleteScriptID = id
	return nil
}

func TestStepCreateSSHKey(t *testing.T) {
	cfg := &Config{
		TemporarySSHKeyName: "packer-key",
	}
	cfg.Comm.Type = "ssh"
	cfg.Comm.SSHPublicKey = []byte("ssh-rsa AAAA test\n")

	client := &fakeClient{}
	state := testState(client)
	step := &stepCreateSSHKey{Config: cfg}

	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("action = %v", action)
	}
	if client.createSSHKeyReq == nil {
		t.Fatal("expected SSH key creation")
	}
	if client.createSSHKeyReq.PublicKey != "ssh-rsa AAAA test" {
		t.Fatalf("PublicKey = %q", client.createSSHKeyReq.PublicKey)
	}
	if got := cfg.SSHKeyIDs[0]; got != "key-1" {
		t.Fatalf("SSHKeyIDs[0] = %q", got)
	}

	step.Cleanup(state)
	if client.deleteSSHKeyID != "key-1" {
		t.Fatalf("deleteSSHKeyID = %q", client.deleteSSHKeyID)
	}
}

func TestStepCreateInstanceRequest(t *testing.T) {
	cfg := &Config{
		InstanceType:      "V100",
		Image:             "ubuntu-24.04",
		Hostname:          "packer-test",
		Description:       "Packer test",
		LocationCode:      "FIN-03",
		Contract:          "PAY_AS_YOU_GO",
		SSHKeyIDs:         []string{"key-1"},
		StartupScriptID:   "script-1",
		OSVolumeName:      "os-volume",
		OSVolumeSize:      100,
		ExistingVolumeIDs: []string{"vol-existing"},
		Volumes:           []Volume{{Name: "data", Size: 50, Type: "NVMe"}},
		VolumeIDsToDelete: []string{"vol-os"},
		DeletePermanently: true,
	}
	client := &fakeClient{}
	state := testState(client)
	step := &stepCreateInstance{Config: cfg}

	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("action = %v", action)
	}
	req := client.createInstanceReq
	if req == nil {
		t.Fatal("expected instance creation")
	}
	if req.StartupScriptID == nil || *req.StartupScriptID != "script-1" {
		t.Fatalf("StartupScriptID = %#v", req.StartupScriptID)
	}
	if req.OSVolume == nil || req.OSVolume.Size != 100 {
		t.Fatalf("OSVolume = %#v", req.OSVolume)
	}
	if len(req.Volumes) != 1 || req.Volumes[0].LocationCode != "FIN-03" {
		t.Fatalf("Volumes = %#v", req.Volumes)
	}

	step.Cleanup(state)
	if client.deleteInstanceID != "inst-1" {
		t.Fatalf("deleteInstanceID = %q", client.deleteInstanceID)
	}
	if !client.deletePermanently {
		t.Fatal("expected deletePermanently")
	}
}

func TestStepCreateOSVolumeArtifactClonesSourceVolume(t *testing.T) {
	clone := true
	ip := "203.0.113.10"
	osVolumeID := "vol-os"
	client := &fakeClient{
		instances: []*verda.Instance{
			{ID: "inst-1", Status: verda.StatusOffline, IP: &ip, OSVolumeID: &osVolumeID},
		},
		volumes: []*verda.Volume{
			{ID: "vol-os", Name: "source-volume", Type: verda.VolumeTypeNVMe, Status: verda.VolumeStatusAttached, Location: "FIN-03"},
			{ID: "vol-cloned", Name: "artifact-volume", Status: verda.VolumeStatusCloning, Location: "FIN-03"},
			{ID: "vol-cloned", Name: "artifact-volume", Status: verda.VolumeStatusDetached, Location: "FIN-03"},
		},
	}
	state := testState(client)
	state.Put(stateKeyInstance, instanceState{ID: "inst-1", IP: ip, Status: verda.StatusRunning, Location: "FIN-03", OSVolumeID: osVolumeID})

	step := &stepCreateOSVolumeArtifact{
		Config: &Config{
			ArtifactType:               artifactTypeOSVolume,
			CloneOSVolume:              &clone,
			ArtifactVolumeName:         "artifact-volume",
			ArtifactVolumeLocationCode: "FIN-03",
			PollInterval:               time.Millisecond,
			InstanceTimeout:            time.Second,
		},
	}
	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("action = %v, err = %v", action, state.Get("error"))
	}
	if client.shutdownInstanceID != "inst-1" {
		t.Fatalf("shutdownInstanceID = %q", client.shutdownInstanceID)
	}
	if client.cloneVolumeSourceID != osVolumeID {
		t.Fatalf("cloneVolumeSourceID = %q", client.cloneVolumeSourceID)
	}
	if client.cloneVolumeReq == nil || client.cloneVolumeReq.Name != "artifact-volume" {
		t.Fatalf("cloneVolumeReq = %#v", client.cloneVolumeReq)
	}
	if client.cloneVolumeReq.LocationCode != "FIN-03" || client.cloneVolumeReq.Type != verda.VolumeTypeNVMe {
		t.Fatalf("cloneVolumeReq = %#v", client.cloneVolumeReq)
	}
	volume, ok := volumeArtifactFromState(state)
	if !ok {
		t.Fatal("expected volume artifact state")
	}
	if volume.ID != "vol-cloned" || !volume.Cloned {
		t.Fatalf("volume artifact = %#v", volume)
	}
}

func TestStepCreateOSVolumeArtifactClonesToMultipleLocations(t *testing.T) {
	clone := true
	ip := "203.0.113.10"
	osVolumeID := "vol-os"
	client := &fakeClient{
		cloneVolumeIDs: []string{"vol-fin-01", "vol-fin-03"},
		instances: []*verda.Instance{
			{ID: "inst-1", Status: verda.StatusOffline, IP: &ip, OSVolumeID: &osVolumeID},
		},
		volumes: []*verda.Volume{
			{ID: "vol-os", Name: "source-volume", Type: verda.VolumeTypeNVMe, Status: verda.VolumeStatusAttached, Location: "FIN-01"},
			{ID: "vol-fin-01", Name: "artifact-volume-fin-01", Status: verda.VolumeStatusCloning, Location: "FIN-01"},
			{ID: "vol-fin-01", Name: "artifact-volume-fin-01", Status: verda.VolumeStatusDetached, Location: "FIN-01"},
			{ID: "vol-fin-03", Name: "artifact-volume-fin-03", Status: verda.VolumeStatusCloning, Location: "FIN-03"},
			{ID: "vol-fin-03", Name: "artifact-volume-fin-03", Status: verda.VolumeStatusDetached, Location: "FIN-03"},
		},
	}
	state := testState(client)
	state.Put(stateKeyInstance, instanceState{ID: "inst-1", IP: ip, Status: verda.StatusRunning, Location: "FIN-01", OSVolumeID: osVolumeID})

	step := &stepCreateOSVolumeArtifact{
		Config: &Config{
			ArtifactType:                artifactTypeOSVolume,
			CloneOSVolume:               &clone,
			ArtifactVolumeName:          "artifact-volume",
			ArtifactVolumeLocationCodes: []string{"FIN-01", "FIN-03"},
			PollInterval:                time.Millisecond,
			InstanceTimeout:             time.Second,
		},
	}
	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("action = %v, err = %v", action, state.Get("error"))
	}
	if len(client.cloneVolumeReqs) != 2 {
		t.Fatalf("cloneVolumeReqs = %#v", client.cloneVolumeReqs)
	}
	if len(client.cloneVolumeSourceIDs) != 2 ||
		client.cloneVolumeSourceIDs[0] != osVolumeID ||
		client.cloneVolumeSourceIDs[1] != "vol-fin-01" {
		t.Fatalf("cloneVolumeSourceIDs = %#v", client.cloneVolumeSourceIDs)
	}
	if client.cloneVolumeReqs[0].Name != "artifact-volume-fin-01" ||
		client.cloneVolumeReqs[0].LocationCode != "FIN-01" ||
		client.cloneVolumeReqs[1].Name != "artifact-volume-fin-03" ||
		client.cloneVolumeReqs[1].LocationCode != "FIN-03" {
		t.Fatalf("cloneVolumeReqs = %#v", client.cloneVolumeReqs)
	}

	volume, ok := volumeArtifactFromState(state)
	if !ok {
		t.Fatal("expected volume artifact state")
	}
	if volume.ID != "vol-fin-01" || volume.Location != "FIN-01" {
		t.Fatalf("volume artifact = %#v", volume)
	}
	if len(volume.Replicas) != 2 || volume.Replicas[1].ID != "vol-fin-03" {
		t.Fatalf("volume replicas = %#v", volume.Replicas)
	}

	generated := state.Get("generated_data").(map[string]interface{})
	idsByLocation := generated["VolumeIDsByLocation"].(map[string]string)
	if idsByLocation["FIN-01"] != "vol-fin-01" || idsByLocation["FIN-03"] != "vol-fin-03" {
		t.Fatalf("VolumeIDsByLocation = %#v", idsByLocation)
	}
}

func TestStepCreateOSVolumeArtifactUsesTemporarySourceLocationSeed(t *testing.T) {
	clone := true
	ip := "203.0.113.10"
	osVolumeID := "vol-os"
	client := &fakeClient{
		cloneVolumeIDs: []string{"vol-seed-fin-03", "vol-fin-01", "vol-fin-02"},
		instances: []*verda.Instance{
			{ID: "inst-1", Status: verda.StatusOffline, IP: &ip, OSVolumeID: &osVolumeID},
		},
		volumes: []*verda.Volume{
			{ID: "vol-os", Name: "source-volume", Type: verda.VolumeTypeNVMe, Status: verda.VolumeStatusAttached, Location: "FIN-03"},
			{ID: "vol-seed-fin-03", Name: "artifact-volume-fin-03-seed", Status: verda.VolumeStatusDetached, Location: "FIN-03"},
			{ID: "vol-fin-01", Name: "artifact-volume-fin-01", Status: verda.VolumeStatusDetached, Location: "FIN-01"},
			{ID: "vol-fin-02", Name: "artifact-volume-fin-02", Status: verda.VolumeStatusDetached, Location: "FIN-02"},
		},
	}
	state := testState(client)
	state.Put(stateKeyInstance, instanceState{ID: "inst-1", IP: ip, Status: verda.StatusRunning, Location: "FIN-03", OSVolumeID: osVolumeID})

	step := &stepCreateOSVolumeArtifact{
		Config: &Config{
			ArtifactType:                artifactTypeOSVolume,
			CloneOSVolume:               &clone,
			ArtifactVolumeName:          "artifact-volume",
			ArtifactVolumeLocationCodes: []string{"FIN-01", "FIN-02"},
			PollInterval:                time.Millisecond,
			InstanceTimeout:             time.Second,
		},
	}
	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("action = %v, err = %v", action, state.Get("error"))
	}
	if len(client.cloneVolumeSourceIDs) != 3 ||
		client.cloneVolumeSourceIDs[0] != osVolumeID ||
		client.cloneVolumeSourceIDs[1] != "vol-seed-fin-03" ||
		client.cloneVolumeSourceIDs[2] != "vol-seed-fin-03" {
		t.Fatalf("cloneVolumeSourceIDs = %#v", client.cloneVolumeSourceIDs)
	}
	if client.cloneVolumeReqs[0].Name != "artifact-volume-fin-03-seed" ||
		client.cloneVolumeReqs[0].LocationCode != "FIN-03" ||
		client.cloneVolumeReqs[1].LocationCode != "FIN-01" ||
		client.cloneVolumeReqs[2].LocationCode != "FIN-02" {
		t.Fatalf("cloneVolumeReqs = %#v", client.cloneVolumeReqs)
	}
	if len(client.deleteVolumeCallIDs) != 1 || client.deleteVolumeCallIDs[0] != "vol-seed-fin-03" {
		t.Fatalf("deleted volumes = %#v", client.deleteVolumeCallIDs)
	}

	volume, ok := volumeArtifactFromState(state)
	if !ok {
		t.Fatal("expected volume artifact state")
	}
	if volume.ID != "vol-fin-01" || volume.Location != "FIN-01" {
		t.Fatalf("volume artifact = %#v", volume)
	}
	if len(volume.Replicas) != 2 ||
		volume.Replicas[0].ID != "vol-fin-01" ||
		volume.Replicas[1].ID != "vol-fin-02" {
		t.Fatalf("volume replicas = %#v", volume.Replicas)
	}
}

func TestStepCreateOSVolumeArtifactCleanupDeletesPartialClones(t *testing.T) {
	clone := true
	ip := "203.0.113.10"
	osVolumeID := "vol-os"
	client := &fakeClient{
		cloneVolumeIDs:  []string{"vol-fin-01"},
		cloneVolumeErrs: []error{nil, errors.New("cross-location clone failed")},
		instances: []*verda.Instance{
			{ID: "inst-1", Status: verda.StatusOffline, IP: &ip, OSVolumeID: &osVolumeID},
		},
		volumes: []*verda.Volume{
			{ID: "vol-os", Name: "source-volume", Type: verda.VolumeTypeNVMe, Status: verda.VolumeStatusAttached, Location: "FIN-01"},
			{ID: "vol-fin-01", Name: "artifact-volume-fin-01", Status: verda.VolumeStatusDetached, Location: "FIN-01"},
		},
	}
	state := testState(client)
	state.Put(stateKeyInstance, instanceState{ID: "inst-1", IP: ip, Status: verda.StatusRunning, Location: "FIN-01", OSVolumeID: osVolumeID})

	step := &stepCreateOSVolumeArtifact{
		Config: &Config{
			ArtifactType:                artifactTypeOSVolume,
			CloneOSVolume:               &clone,
			ArtifactVolumeName:          "artifact-volume",
			ArtifactVolumeLocationCodes: []string{"FIN-01", "FIN-03"},
			PollInterval:                time.Millisecond,
			InstanceTimeout:             time.Second,
		},
	}
	if action := step.Run(context.Background(), state); action != multistep.ActionHalt {
		t.Fatalf("action = %v, err = %v", action, state.Get("error"))
	}

	step.Cleanup(state)
	if len(client.deleteVolumeCallIDs) != 1 || client.deleteVolumeCallIDs[0] != "vol-fin-01" {
		t.Fatalf("deleted volumes = %#v", client.deleteVolumeCallIDs)
	}
}

func TestStepCreateInstanceCleanupPreservesUnclonedOSVolumeArtifact(t *testing.T) {
	clone := false
	client := &fakeClient{}
	state := testState(client)
	state.Put(stateKeyInstance, instanceState{ID: "inst-1", OSVolumeID: "vol-os"})
	saveVolumeArtifactState(state, volumeArtifactState{ID: "vol-os", SourceOSVolumeID: "vol-os"})

	step := &stepCreateInstance{
		Config: &Config{
			ArtifactType:  artifactTypeOSVolume,
			CloneOSVolume: &clone,
		},
	}
	step.Cleanup(state)
	if client.deleteInstanceID != "inst-1" {
		t.Fatalf("deleteInstanceID = %q", client.deleteInstanceID)
	}
	if client.deleteVolumeIDs == nil {
		t.Fatal("expected empty deleteVolumeIDs slice, got nil")
	}
	if len(client.deleteVolumeIDs) != 0 {
		t.Fatalf("deleteVolumeIDs = %#v", client.deleteVolumeIDs)
	}
}

func TestStepWaitForInstance(t *testing.T) {
	ip := "203.0.113.10"
	client := &fakeClient{
		instances: []*verda.Instance{
			{ID: "inst-1", Status: "provisioning"},
			{ID: "inst-1", Status: "running", IP: &ip, InstanceType: "V100", Location: "FIN-03"},
		},
	}
	state := testState(client)
	state.Put(stateKeyInstance, instanceState{ID: "inst-1"})

	step := &stepWaitForInstance{
		Config: &Config{
			PollInterval:       time.Millisecond,
			InstanceTimeout:    time.Second,
			AllowedSSHStatuses: []string{"running"},
		},
	}
	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("action = %v", action)
	}
	if got := state.Get(stateKeyInstanceIP).(string); got != ip {
		t.Fatalf("instance ip = %q", got)
	}
}

func testState(client verdaClient) multistep.StateBag {
	state := new(multistep.BasicStateBag)
	state.Put("ui", &packer.MockUi{})
	putClient(state, client)
	return state
}
