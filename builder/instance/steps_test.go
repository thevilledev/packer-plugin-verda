package instance

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

type fakeClient struct {
	createSSHKeyReq       *verda.CreateSSHKeyRequest
	deleteSSHKeyID        string
	createScriptReq       *verda.CreateStartupScriptRequest
	deleteScriptID        string
	createInstanceReq     *verda.CreateInstanceRequest
	deleteInstanceID      string
	deleteVolumeIDs       []string
	deletePermanently     bool
	shutdownInstanceID    string
	cloneVolumeSourceID   string
	cloneVolumeSourceIDs  []string
	cloneVolumeReq        *volumeCloneRequest
	cloneVolumeReqs       []volumeCloneRequest
	cloneVolumeIDs        []string
	cloneVolumeErrs       []error
	deleteVolumeID        string
	deleteVolumeForce     bool
	deleteVolumeCallIDs   []string
	deleteInstanceCallIDs []string
	instancesByID         map[string][]*verda.Instance
	volumesByID           map[string][]*verda.Volume
	getInstanceCallsByID  map[string]int
	getVolumeCallsByID    map[string]int
	createInstanceErr     error
	deleteInstanceErr     error
	getVolumeErr          error
	deleteVolumeErrs      map[string]error
	createInstanceFn      func(context.Context, verda.CreateInstanceRequest) (*verda.Instance, error)
	createInstanceReqs    []verda.CreateInstanceRequest
	createSSHKeyIDs       []string
	createSSHKeyCount     int
}

func (f *fakeClient) GetInstance(_ context.Context, id string) (*verda.Instance, error) {
	instances := f.instancesByID[id]
	if len(instances) == 0 {
		return nil, errors.New("no fake instances configured")
	}
	if f.getInstanceCallsByID == nil {
		f.getInstanceCallsByID = make(map[string]int)
	}
	idx := f.getInstanceCallsByID[id]
	if idx >= len(instances) {
		idx = len(instances) - 1
	}
	f.getInstanceCallsByID[id]++
	return instances[idx], nil
}

func (f *fakeClient) CreateInstance(ctx context.Context, req verda.CreateInstanceRequest) (*verda.Instance, error) {
	f.createInstanceReq = &req
	f.createInstanceReqs = append(f.createInstanceReqs, req)
	if f.createInstanceFn != nil {
		return f.createInstanceFn(ctx, req)
	}
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
	f.deleteInstanceCallIDs = append(f.deleteInstanceCallIDs, id)
	f.deleteVolumeIDs = volumeIDs
	f.deletePermanently = deletePermanently
	return f.deleteInstanceErr
}

func (f *fakeClient) ShutdownInstance(_ context.Context, id string) error {
	f.shutdownInstanceID = id
	return nil
}

func (f *fakeClient) GetVolume(_ context.Context, id string) (*verda.Volume, error) {
	if f.getVolumeErr != nil {
		return nil, f.getVolumeErr
	}
	volumes := f.volumesByID[id]
	if len(volumes) == 0 {
		return &verda.Volume{
			ID:       id,
			Name:     "volume-" + id,
			Status:   verda.VolumeStatusDetached,
			Location: "FIN-03",
		}, nil
	}
	if f.getVolumeCallsByID == nil {
		f.getVolumeCallsByID = make(map[string]int)
	}
	idx := f.getVolumeCallsByID[id]
	if idx >= len(volumes) {
		idx = len(volumes) - 1
	}
	f.getVolumeCallsByID[id]++
	return volumes[idx], nil
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
	return f.deleteVolumeErrs[id]
}

func (f *fakeClient) CreateSSHKey(_ context.Context, req verda.CreateSSHKeyRequest) (*verda.SSHKey, error) {
	f.createSSHKeyReq = &req
	id := "key-1"
	if len(f.createSSHKeyIDs) > 0 {
		idx := f.createSSHKeyCount
		if idx >= len(f.createSSHKeyIDs) {
			idx = len(f.createSSHKeyIDs) - 1
		}
		id = f.createSSHKeyIDs[idx]
	}
	f.createSSHKeyCount++
	return &verda.SSHKey{ID: id, Name: req.Name}, nil
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
	if client.createSSHKeyReq.PublicKey != "ssh-rsa AAAA packer-key" {
		t.Fatalf("PublicKey = %q", client.createSSHKeyReq.PublicKey)
	}
	if got := createdSSHKeyIDFromState(state); got != "key-1" {
		t.Fatalf("created SSH key ID = %q", got)
	}
	if len(cfg.SSHKeyIDs) != 0 {
		t.Fatalf("SSHKeyIDs mutated to %#v", cfg.SSHKeyIDs)
	}

	step.Cleanup(state)
	if client.deleteSSHKeyID != "key-1" {
		t.Fatalf("deleteSSHKeyID = %q", client.deleteSSHKeyID)
	}
}

func TestStepCreateSSHKeyWithNoneCommunicator(t *testing.T) {
	cfg := &Config{
		TemporarySSHKeyName: "packer-key",
	}
	cfg.Comm.Type = "none"
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
	if got := createdSSHKeyIDFromState(state); got != "key-1" {
		t.Fatalf("created SSH key ID = %q", got)
	}
}

func TestStepCreateStartupScriptStoresRunStateWithoutMutatingConfig(t *testing.T) {
	cfg := &Config{
		StartupScript:       "#!/bin/sh\necho ready\n",
		StartupScriptName:   "packer-startup",
		DeleteStartupScript: true,
	}
	client := &fakeClient{}
	state := testState(client)
	step := &stepCreateStartupScript{Config: cfg}

	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("action = %v", action)
	}
	if got := createdStartupScriptIDFromState(state); got != "script-1" {
		t.Fatalf("created startup script ID = %q", got)
	}
	if cfg.StartupScriptID != "" {
		t.Fatalf("StartupScriptID mutated to %q", cfg.StartupScriptID)
	}
	req := cfg.instanceRequest("", createdStartupScriptIDFromState(state))
	if req.StartupScriptID == nil || *req.StartupScriptID != "script-1" {
		t.Fatalf("request StartupScriptID = %#v", req.StartupScriptID)
	}

	step.Cleanup(state)
	if client.deleteScriptID != "script-1" {
		t.Fatalf("deleteScriptID = %q", client.deleteScriptID)
	}
}

func TestStepCreateInstanceRequest(t *testing.T) {
	cfg := &Config{
		InstanceType:         "V100",
		Image:                "ubuntu-24.04",
		Hostname:             "packer-test",
		Description:          "Packer test",
		LocationCode:         "FIN-03",
		Contract:             spotContract,
		Pricing:              "FIXED_PRICE",
		IsSpot:               true,
		Coupon:               "PACKER20",
		SSHKeyIDs:            []string{"key-1"},
		StartupScriptID:      "script-1",
		OSVolumeName:         "os-volume",
		OSVolumeSize:         100,
		OSVolumeSpotBehavior: verda.SpotDiscontinueKeepDetached,
		ExistingVolumeIDs:    []string{"vol-existing"},
		Volumes: []Volume{{
			Name:              "data",
			Size:              50,
			Type:              "NVMe",
			OnSpotDiscontinue: verda.SpotDiscontinueMoveToTrash,
		}},
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
	if req.OSVolume.OnSpotDiscontinue != verda.SpotDiscontinueKeepDetached {
		t.Fatalf("OSVolume OnSpotDiscontinue = %q", req.OSVolume.OnSpotDiscontinue)
	}
	if len(req.Volumes) != 1 || req.Volumes[0].LocationCode != "FIN-03" {
		t.Fatalf("Volumes = %#v", req.Volumes)
	}
	if !req.IsSpot || req.Contract != spotContract {
		t.Fatalf("spot request fields: IsSpot=%t Contract=%q", req.IsSpot, req.Contract)
	}
	if req.Pricing != "FIXED_PRICE" {
		t.Fatalf("Pricing = %q", req.Pricing)
	}
	if req.Coupon == nil || *req.Coupon != "PACKER20" {
		t.Fatalf("Coupon = %#v", req.Coupon)
	}
	if req.Volumes[0].OnSpotDiscontinue != verda.SpotDiscontinueMoveToTrash {
		t.Fatalf("volume OnSpotDiscontinue = %q", req.Volumes[0].OnSpotDiscontinue)
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
		instancesByID: map[string][]*verda.Instance{
			"inst-1": {{ID: "inst-1", Status: verda.StatusOffline, IP: &ip, OSVolumeID: &osVolumeID}},
		},
		volumesByID: map[string][]*verda.Volume{
			"vol-os": {{ID: "vol-os", Name: "source-volume", Type: verda.VolumeTypeNVMe, Status: verda.VolumeStatusAttached, Location: "FIN-03"}},
			"vol-cloned": {
				{ID: "vol-cloned", Name: "artifact-volume", Status: verda.VolumeStatusCloning, Location: "FIN-03"},
				{ID: "vol-cloned", Name: "artifact-volume", Status: verda.VolumeStatusDetached, Location: "FIN-03"},
			},
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
		instancesByID: map[string][]*verda.Instance{
			"inst-1": {{ID: "inst-1", Status: verda.StatusOffline, IP: &ip, OSVolumeID: &osVolumeID}},
		},
		volumesByID: map[string][]*verda.Volume{
			"vol-os": {{ID: "vol-os", Name: "source-volume", Type: verda.VolumeTypeNVMe, Status: verda.VolumeStatusAttached, Location: "FIN-01"}},
			"vol-fin-01": {
				{ID: "vol-fin-01", Name: "artifact-volume", Status: verda.VolumeStatusCloning, Location: "FIN-01"},
				{ID: "vol-fin-01", Name: "artifact-volume", Status: verda.VolumeStatusDetached, Location: "FIN-01"},
			},
			"vol-fin-03": {
				{ID: "vol-fin-03", Name: "artifact-volume", Status: verda.VolumeStatusCloning, Location: "FIN-03"},
				{ID: "vol-fin-03", Name: "artifact-volume", Status: verda.VolumeStatusDetached, Location: "FIN-03"},
			},
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
	if client.cloneVolumeReqs[0].Name != "artifact-volume" ||
		client.cloneVolumeReqs[0].LocationCode != "FIN-01" ||
		client.cloneVolumeReqs[1].Name != "artifact-volume" ||
		client.cloneVolumeReqs[1].LocationCode != "FIN-03" {
		t.Fatalf("cloneVolumeReqs = %#v", client.cloneVolumeReqs)
	}
	if calls := client.getVolumeCallsByID["vol-fin-01"]; calls != 2 {
		t.Fatalf("source-location clone lookups = %d, want 2 before fan-out", calls)
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

func TestStepCreateOSVolumeArtifactRetainsSourceLocationReplica(t *testing.T) {
	clone := true
	ip := "203.0.113.10"
	osVolumeID := "vol-os"
	client := &fakeClient{
		cloneVolumeIDs: []string{"vol-fin-03", "vol-fin-01", "vol-fin-02"},
		instancesByID: map[string][]*verda.Instance{
			"inst-1": {{ID: "inst-1", Status: verda.StatusOffline, IP: &ip, OSVolumeID: &osVolumeID}},
		},
		volumesByID: map[string][]*verda.Volume{
			"vol-os":     {{ID: "vol-os", Name: "source-volume", Type: verda.VolumeTypeNVMe, Status: verda.VolumeStatusAttached, Location: "FIN-03"}},
			"vol-fin-03": {{ID: "vol-fin-03", Name: "artifact-volume", Status: verda.VolumeStatusDetached, Location: "FIN-03"}},
			"vol-fin-01": {{ID: "vol-fin-01", Name: "artifact-volume", Status: verda.VolumeStatusDetached, Location: "FIN-01"}},
			"vol-fin-02": {{ID: "vol-fin-02", Name: "artifact-volume", Status: verda.VolumeStatusDetached, Location: "FIN-02"}},
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
		client.cloneVolumeSourceIDs[1] != "vol-fin-03" ||
		client.cloneVolumeSourceIDs[2] != "vol-fin-03" {
		t.Fatalf("cloneVolumeSourceIDs = %#v", client.cloneVolumeSourceIDs)
	}
	if client.cloneVolumeReqs[0].Name != "artifact-volume" ||
		client.cloneVolumeReqs[0].LocationCode != "FIN-03" ||
		client.cloneVolumeReqs[1].Name != "artifact-volume" ||
		client.cloneVolumeReqs[1].LocationCode != "FIN-01" ||
		client.cloneVolumeReqs[2].Name != "artifact-volume" ||
		client.cloneVolumeReqs[2].LocationCode != "FIN-02" {
		t.Fatalf("cloneVolumeReqs = %#v", client.cloneVolumeReqs)
	}
	if len(client.deleteVolumeCallIDs) != 0 {
		t.Fatalf("deleted volumes = %#v", client.deleteVolumeCallIDs)
	}

	volume, ok := volumeArtifactFromState(state)
	if !ok {
		t.Fatal("expected volume artifact state")
	}
	if volume.ID != "vol-fin-01" || volume.Location != "FIN-01" {
		t.Fatalf("volume artifact = %#v", volume)
	}
	if len(volume.Replicas) != 3 ||
		volume.Replicas[0].ID != "vol-fin-01" ||
		volume.Replicas[1].ID != "vol-fin-02" ||
		volume.Replicas[2].ID != "vol-fin-03" {
		t.Fatalf("volume replicas = %#v", volume.Replicas)
	}

	generated := state.Get("generated_data").(map[string]interface{})
	idsByLocation := generated["VolumeIDsByLocation"].(map[string]string)
	if idsByLocation["FIN-01"] != "vol-fin-01" ||
		idsByLocation["FIN-02"] != "vol-fin-02" ||
		idsByLocation["FIN-03"] != "vol-fin-03" {
		t.Fatalf("VolumeIDsByLocation = %#v", idsByLocation)
	}
}

func TestStepCreateOSVolumeArtifactAddsSourceReplicaToConfiguredLocations(t *testing.T) {
	clone := true
	ip := "203.0.113.10"
	osVolumeID := "vol-os"
	client := &fakeClient{
		cloneVolumeIDs: []string{"vol-fin-03", "vol-fin-02"},
		instancesByID: map[string][]*verda.Instance{
			"inst-1": {{ID: "inst-1", Status: verda.StatusOffline, IP: &ip, OSVolumeID: &osVolumeID}},
		},
		volumesByID: map[string][]*verda.Volume{
			"vol-os":     {{ID: "vol-os", Name: "source-volume", Type: verda.VolumeTypeNVMe, Status: verda.VolumeStatusAttached, Location: "FIN-03"}},
			"vol-fin-03": {{ID: "vol-fin-03", Name: "artifact-volume", Status: verda.VolumeStatusDetached, Location: "FIN-03"}},
			"vol-fin-02": {{ID: "vol-fin-02", Name: "artifact-volume", Status: verda.VolumeStatusDetached, Location: "FIN-02"}},
		},
	}
	state := testState(client)
	state.Put(stateKeyInstance, instanceState{ID: "inst-1", IP: ip, Status: verda.StatusRunning, Location: "FIN-03", OSVolumeID: osVolumeID})

	step := &stepCreateOSVolumeArtifact{
		Config: &Config{
			ArtifactType:                artifactTypeOSVolume,
			CloneOSVolume:               &clone,
			ArtifactVolumeName:          "artifact-volume",
			ArtifactVolumeLocationCodes: []string{"FIN-02"},
			LocationCode:                "FIN-03",
			PollInterval:                time.Millisecond,
			InstanceTimeout:             time.Second,
		},
	}
	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("action = %v, err = %v", action, state.Get("error"))
	}
	if len(client.cloneVolumeSourceIDs) != 2 ||
		client.cloneVolumeSourceIDs[0] != osVolumeID ||
		client.cloneVolumeSourceIDs[1] != "vol-fin-03" {
		t.Fatalf("cloneVolumeSourceIDs = %#v", client.cloneVolumeSourceIDs)
	}
	if len(client.cloneVolumeReqs) != 2 ||
		client.cloneVolumeReqs[0].Name != "artifact-volume" ||
		client.cloneVolumeReqs[0].LocationCode != "FIN-03" ||
		client.cloneVolumeReqs[1].Name != "artifact-volume" ||
		client.cloneVolumeReqs[1].LocationCode != "FIN-02" {
		t.Fatalf("cloneVolumeReqs = %#v", client.cloneVolumeReqs)
	}
	if len(client.deleteVolumeCallIDs) != 0 {
		t.Fatalf("deleted volumes = %#v", client.deleteVolumeCallIDs)
	}

	volume, ok := volumeArtifactFromState(state)
	if !ok {
		t.Fatal("expected volume artifact state")
	}
	if volume.ID != "vol-fin-02" || volume.Location != "FIN-02" {
		t.Fatalf("volume artifact = %#v", volume)
	}
	if len(volume.Replicas) != 2 ||
		volume.Replicas[0].ID != "vol-fin-02" ||
		volume.Replicas[1].ID != "vol-fin-03" {
		t.Fatalf("volume replicas = %#v", volume.Replicas)
	}

	generated := state.Get("generated_data").(map[string]interface{})
	idsByLocation := generated["VolumeIDsByLocation"].(map[string]string)
	if idsByLocation["FIN-02"] != "vol-fin-02" || idsByLocation["FIN-03"] != "vol-fin-03" {
		t.Fatalf("VolumeIDsByLocation = %#v", idsByLocation)
	}
}

func TestStepCreateOSVolumeArtifactCleanupDeletesPartialClones(t *testing.T) {
	clone := true
	ip := "203.0.113.10"
	osVolumeID := "vol-os"
	client := &fakeClient{
		cloneVolumeIDs:  []string{"vol-fin-01", "vol-fin-02"},
		cloneVolumeErrs: []error{nil, nil, errors.New("cross-location clone failed")},
		instancesByID: map[string][]*verda.Instance{
			"inst-1": {{ID: "inst-1", Status: verda.StatusOffline, IP: &ip, OSVolumeID: &osVolumeID}},
		},
		volumesByID: map[string][]*verda.Volume{
			"vol-os":     {{ID: "vol-os", Name: "source-volume", Type: verda.VolumeTypeNVMe, Status: verda.VolumeStatusAttached, Location: "FIN-01"}},
			"vol-fin-01": {{ID: "vol-fin-01", Name: "artifact-volume", Status: verda.VolumeStatusDetached, Location: "FIN-01"}},
		},
	}
	state := testState(client)
	state.Put(stateKeyInstance, instanceState{ID: "inst-1", IP: ip, Status: verda.StatusRunning, Location: "FIN-01", OSVolumeID: osVolumeID})

	step := &stepCreateOSVolumeArtifact{
		Config: &Config{
			ArtifactType:                artifactTypeOSVolume,
			CloneOSVolume:               &clone,
			ArtifactVolumeName:          "artifact-volume",
			ArtifactVolumeLocationCodes: []string{"FIN-01", "FIN-02", "FIN-03"},
			PollInterval:                time.Millisecond,
			InstanceTimeout:             time.Second,
		},
	}
	if action := step.Run(context.Background(), state); action != multistep.ActionHalt {
		t.Fatalf("action = %v, err = %v", action, state.Get("error"))
	}
	if len(client.cloneVolumeSourceIDs) != 3 ||
		client.cloneVolumeSourceIDs[0] != osVolumeID ||
		client.cloneVolumeSourceIDs[1] != "vol-fin-01" ||
		client.cloneVolumeSourceIDs[2] != "vol-fin-01" {
		t.Fatalf("cloneVolumeSourceIDs = %#v", client.cloneVolumeSourceIDs)
	}
	if _, ok := volumeArtifactFromState(state); ok {
		t.Fatal("unexpected promoted volume artifact after clone failure")
	}

	step.Cleanup(state)
	if !reflect.DeepEqual(client.deleteVolumeCallIDs, []string{"vol-fin-01", "vol-fin-02"}) {
		t.Fatalf("deleted volumes = %#v", client.deleteVolumeCallIDs)
	}
}

func TestArtifactVolumeLocationsWithSource(t *testing.T) {
	tests := []struct {
		name      string
		locations []string
		source    string
		want      []string
	}{
		{
			name:      "append implicit source",
			locations: []string{"FIN-02"},
			source:    "FIN-03",
			want:      []string{"FIN-02", "FIN-03"},
		},
		{
			name:      "explicit source remains in place",
			locations: []string{"FIN-02", "FIN-03"},
			source:    "FIN-03",
			want:      []string{"FIN-02", "FIN-03"},
		},
		{
			name:      "source comparison is case insensitive",
			locations: []string{"fin-03", "FIN-02"},
			source:    "FIN-03",
			want:      []string{"fin-03", "FIN-02"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := artifactVolumeLocationsWithSource(tt.locations, tt.source)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("locations = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestStepCreateOSVolumeArtifactPropagatesUnclonedVolumeError(t *testing.T) {
	clone := false
	client := &fakeClient{getVolumeErr: errors.New("volume lookup failed")}
	state := testState(client)
	state.Put(stateKeyInstance, instanceState{
		ID:         "inst-1",
		Status:     verda.StatusOffline,
		Location:   "FIN-03",
		OSVolumeID: "vol-os",
	})

	step := &stepCreateOSVolumeArtifact{Config: &Config{
		ArtifactType:               artifactTypeOSVolume,
		CloneOSVolume:              &clone,
		SkipShutdownBeforeArtifact: true,
		PollInterval:               time.Millisecond,
		InstanceTimeout:            time.Second,
	}}
	if action := step.Run(context.Background(), state); action != multistep.ActionHalt {
		t.Fatalf("action = %v", action)
	}
	if _, ok := state.GetOk("error"); !ok {
		t.Fatal("expected volume lookup error")
	}
	if _, ok := volumeArtifactFromState(state); ok {
		t.Fatal("unexpected volume artifact state after failure")
	}
}

func TestStepCreateOSVolumeArtifactCleanupDeletesUnpromotedClones(t *testing.T) {
	clone := true
	client := &fakeClient{}
	state := testState(client)
	state.Put(stateKeyCreatedArtifactVolumeIDs, []string{"vol-cloned"})
	saveVolumeArtifactState(state, volumeArtifactState{ID: "vol-cloned", Cloned: true})

	step := &stepCreateOSVolumeArtifact{Config: &Config{
		ArtifactType:  artifactTypeOSVolume,
		CloneOSVolume: &clone,
	}}
	step.Cleanup(state)
	if len(client.deleteVolumeCallIDs) != 1 || client.deleteVolumeCallIDs[0] != "vol-cloned" {
		t.Fatalf("deleted volumes = %#v", client.deleteVolumeCallIDs)
	}
}

func TestStepCreateInstanceCleanupPreservesUnclonedOSVolumeArtifact(t *testing.T) {
	clone := false
	client := &fakeClient{}
	state := testState(client)
	state.Put(stateKeyInstance, instanceState{ID: "inst-1", OSVolumeID: "vol-os"})
	saveVolumeArtifactState(state, volumeArtifactState{ID: "vol-os", SourceOSVolumeID: "vol-os"})
	state.Put(stateKeyBuildComplete, true)

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

func TestStepCreateInstanceCleanupDeletesIncompleteUnclonedOSVolume(t *testing.T) {
	clone := false
	client := &fakeClient{}
	state := testState(client)
	state.Put(stateKeyInstance, instanceState{ID: "inst-1", OSVolumeID: "vol-os"})
	saveVolumeArtifactState(state, volumeArtifactState{ID: "vol-os", SourceOSVolumeID: "vol-os"})

	step := &stepCreateInstance{Config: &Config{
		ArtifactType:  artifactTypeOSVolume,
		CloneOSVolume: &clone,
	}}
	step.Cleanup(state)
	if client.deleteInstanceID != "inst-1" {
		t.Fatalf("deleteInstanceID = %q", client.deleteInstanceID)
	}
	if client.deleteVolumeIDs != nil {
		t.Fatalf("deleteVolumeIDs = %#v, want nil API default", client.deleteVolumeIDs)
	}
}

func TestStepWaitForInstance(t *testing.T) {
	ip := "203.0.113.10"
	client := &fakeClient{
		instancesByID: map[string][]*verda.Instance{
			"inst-1": {
				{ID: "inst-1", Status: "provisioning"},
				{ID: "inst-1", Status: "running", IP: &ip, InstanceType: "V100", Location: "FIN-03"},
			},
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

func TestStepWaitForInstanceWithNoneCommunicatorDoesNotRequireIP(t *testing.T) {
	client := &fakeClient{instancesByID: map[string][]*verda.Instance{
		"inst-1": {{ID: "inst-1", Status: verda.StatusRunning}},
	}}
	state := testState(client)
	state.Put(stateKeyInstance, instanceState{ID: "inst-1"})
	cfg := &Config{
		PollInterval:       time.Millisecond,
		InstanceTimeout:    time.Second,
		AllowedSSHStatuses: []string{verda.StatusRunning},
	}
	cfg.Comm.Type = "none"

	if action := (&stepWaitForInstance{Config: cfg}).Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("action = %v, err = %v", action, state.Get("error"))
	}
}

func TestStepWaitForInstanceRejectsTerminalAllowedStatus(t *testing.T) {
	ip := "203.0.113.10"
	client := &fakeClient{instancesByID: map[string][]*verda.Instance{
		"inst-1": {{ID: "inst-1", Status: verda.StatusError, IP: &ip}},
	}}
	state := testState(client)
	state.Put(stateKeyInstance, instanceState{ID: "inst-1"})
	cfg := &Config{
		PollInterval:       time.Millisecond,
		InstanceTimeout:    time.Second,
		AllowedSSHStatuses: []string{verda.StatusError},
	}
	cfg.Comm.Type = "ssh"

	if action := (&stepWaitForInstance{Config: cfg}).Run(context.Background(), state); action != multistep.ActionHalt {
		t.Fatalf("action = %v", action)
	}
}

func TestPollUntilCancellationAndTimeout(t *testing.T) {
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := pollUntil(
			ctx,
			time.Second,
			time.Millisecond,
			func(context.Context) (string, bool, error) { return "", false, nil },
			func() error { return errors.New("timed out") },
		)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		want := errors.New("timed out")
		_, err := pollUntil(
			context.Background(),
			5*time.Millisecond,
			time.Millisecond,
			func(context.Context) (string, bool, error) { return "", false, nil },
			func() error { return want },
		)
		if !errors.Is(err, want) {
			t.Fatalf("error = %v, want timeout error", err)
		}
	})
}

func testState(client verdaClient) multistep.StateBag {
	state := new(multistep.BasicStateBag)
	state.Put("ui", &packer.MockUi{})
	putClient(state, client)
	return state
}
