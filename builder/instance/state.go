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
	stateKeyBuildComplete            = "verda_build_complete"
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

func instanceFromState(state multistep.StateBag) (instanceState, bool) {
	value, ok := state.GetOk(stateKeyInstance)
	if !ok {
		return instanceState{}, false
	}
	instance, ok := value.(instanceState)
	return instance, ok
}

func buildComplete(state multistep.StateBag) bool {
	complete, _ := state.Get(stateKeyBuildComplete).(bool)
	return complete
}

func createdSSHKeyIDFromState(state multistep.StateBag) string {
	id, _ := state.Get(stateKeyCreatedSSHKeyID).(string)
	return id
}

func createdStartupScriptIDFromState(state multistep.StateBag) string {
	id, _ := state.Get(stateKeyCreatedScriptID).(string)
	return id
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
	summary := summarizeVolumeReplicas(replicas)
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
		"VolumeIDs":                summary.IDs,
		"VolumeIDsByLocation":      summary.IDsByLocation,
		"SourceOSVolumeID":         volume.SourceOSVolumeID,
		"VolumeName":               volume.Name,
		"VolumeNamesByLocation":    summary.NamesByLocation,
		"VolumeLocation":           volume.Location,
		"VolumeLocations":          summary.Locations,
		"VolumeStatus":             volume.Status,
		"VolumeStatusesByLocation": summary.StatusesByLocation,
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

type volumeReplicaSummary struct {
	IDs                []string
	Locations          []string
	IDsByLocation      map[string]string
	NamesByLocation    map[string]string
	StatusesByLocation map[string]string
}

func summarizeVolumeReplicas(replicas []volumeReplicaState) volumeReplicaSummary {
	summary := volumeReplicaSummary{
		IDs:                make([]string, 0, len(replicas)),
		Locations:          make([]string, 0, len(replicas)),
		IDsByLocation:      make(map[string]string, len(replicas)),
		NamesByLocation:    make(map[string]string, len(replicas)),
		StatusesByLocation: make(map[string]string, len(replicas)),
	}
	for _, replica := range replicas {
		summary.IDs = append(summary.IDs, replica.ID)
		summary.Locations = append(summary.Locations, replica.Location)
		summary.IDsByLocation[replica.Location] = replica.ID
		summary.NamesByLocation[replica.Location] = replica.Name
		summary.StatusesByLocation[replica.Location] = replica.Status
	}
	return summary
}
