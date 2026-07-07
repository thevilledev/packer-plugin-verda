package instance

import (
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

const (
	stateKeyClient                   = "verda_client"
	stateKeyCreatedSSHKeyID          = "verda_created_ssh_key_id"
	stateKeyCreatedScriptID          = "verda_created_startup_script_id"
	stateKeyCreatedArtifactVolumeIDs = "verda_created_artifact_volume_ids"
	stateKeyInstance                 = "verda_instance"
	stateKeyInstanceIP               = "verda_instance_ip"
	stateKeyArtifactVolume           = "verda_artifact_volume"
)

type instanceState struct {
	ID           string
	IP           string
	Status       string
	Location     string
	InstanceType string
	OSVolumeID   string
}

type volumeArtifactState struct {
	ID               string
	SourceOSVolumeID string
	Name             string
	Location         string
	Status           string
	Cloned           bool
	Replicas         []volumeReplicaState
}

type volumeReplicaState struct {
	ID       string
	Name     string
	Location string
	Status   string
	Cloned   bool
}

func clientFromState(state multistep.StateBag) verdaClient {
	client, _ := state.Get(stateKeyClient).(verdaClient)
	return client
}

func instanceFromState(state multistep.StateBag) (instanceState, bool) {
	value, ok := state.GetOk(stateKeyInstance)
	if !ok {
		return instanceState{}, false
	}
	instance, ok := value.(instanceState)
	return instance, ok
}

func instanceStateFromSDK(instance *verda.Instance) instanceState {
	out := instanceState{
		ID:           instance.ID,
		Status:       instance.Status,
		Location:     instance.Location,
		InstanceType: instance.InstanceType,
	}
	if instance.IP != nil {
		out.IP = *instance.IP
	}
	if instance.OSVolumeID != nil {
		out.OSVolumeID = *instance.OSVolumeID
	}
	return out
}

func saveInstanceState(state multistep.StateBag, instance *verda.Instance) instanceState {
	current := instanceStateFromSDK(instance)
	state.Put(stateKeyInstance, current)
	state.Put("instance_id", current.ID)
	if current.IP != "" {
		state.Put(stateKeyInstanceIP, current.IP)
	}
	state.Put("generated_data", generatedData(current))
	return current
}

func generatedData(current instanceState) map[string]interface{} {
	return map[string]interface{}{
		"ID":             current.ID,
		"ArtifactType":   artifactTypeInstance,
		"InstanceID":     current.ID,
		"InstanceIP":     current.IP,
		"InstanceStatus": current.Status,
		"InstanceType":   current.InstanceType,
		"Location":       current.Location,
		"OSVolumeID":     current.OSVolumeID,
	}
}

func saveVolumeArtifactState(state multistep.StateBag, volume volumeArtifactState) {
	state.Put(stateKeyArtifactVolume, volume)
	current, _ := instanceFromState(state)
	state.Put("generated_data", generatedDataForArtifact(current, volume))
}

func volumeArtifactFromState(state multistep.StateBag) (volumeArtifactState, bool) {
	value, ok := state.GetOk(stateKeyArtifactVolume)
	if !ok {
		return volumeArtifactState{}, false
	}
	volume, ok := value.(volumeArtifactState)
	return volume, ok
}

func generatedDataForArtifact(current instanceState, volume volumeArtifactState) map[string]interface{} {
	replicas := normalizedVolumeReplicas(volume)
	return map[string]interface{}{
		"ID":                       volume.ID,
		"ArtifactType":             artifactTypeOSVolume,
		"InstanceID":               current.ID,
		"InstanceIP":               current.IP,
		"InstanceStatus":           current.Status,
		"InstanceType":             current.InstanceType,
		"Location":                 current.Location,
		"OSVolumeID":               current.OSVolumeID,
		"VolumeID":                 volume.ID,
		"VolumeIDs":                volumeReplicaIDs(replicas),
		"VolumeIDsByLocation":      volumeReplicaIDsByLocation(replicas),
		"SourceOSVolumeID":         volume.SourceOSVolumeID,
		"VolumeName":               volume.Name,
		"VolumeNamesByLocation":    volumeReplicaNamesByLocation(replicas),
		"VolumeLocation":           volume.Location,
		"VolumeLocations":          volumeReplicaLocations(replicas),
		"VolumeStatus":             volume.Status,
		"VolumeStatusesByLocation": volumeReplicaStatusesByLocation(replicas),
		"VolumeCloned":             volume.Cloned,
	}
}

func normalizedVolumeReplicas(volume volumeArtifactState) []volumeReplicaState {
	if len(volume.Replicas) > 0 {
		return volume.Replicas
	}
	return []volumeReplicaState{{
		ID:       volume.ID,
		Name:     volume.Name,
		Location: volume.Location,
		Status:   volume.Status,
		Cloned:   volume.Cloned,
	}}
}

func volumeReplicaIDs(replicas []volumeReplicaState) []string {
	ids := make([]string, 0, len(replicas))
	for _, replica := range replicas {
		ids = append(ids, replica.ID)
	}
	return ids
}

func volumeReplicaLocations(replicas []volumeReplicaState) []string {
	locations := make([]string, 0, len(replicas))
	for _, replica := range replicas {
		locations = append(locations, replica.Location)
	}
	return locations
}

func volumeReplicaIDsByLocation(replicas []volumeReplicaState) map[string]string {
	ids := make(map[string]string, len(replicas))
	for _, replica := range replicas {
		ids[replica.Location] = replica.ID
	}
	return ids
}

func volumeReplicaNamesByLocation(replicas []volumeReplicaState) map[string]string {
	names := make(map[string]string, len(replicas))
	for _, replica := range replicas {
		names[replica.Location] = replica.Name
	}
	return names
}

func volumeReplicaStatusesByLocation(replicas []volumeReplicaState) map[string]string {
	statuses := make(map[string]string, len(replicas))
	for _, replica := range replicas {
		statuses[replica.Location] = replica.Status
	}
	return statuses
}
