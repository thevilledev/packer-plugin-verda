package instance

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// DeleteConfig controls artifact destruction behavior.
type DeleteConfig struct {
	VolumeIDs         []string
	DeletePermanently bool
	DeleteOnDestroy   bool
}

// Artifact describes the Verda resource produced by a build.
type Artifact struct {
	Client              verdaClient
	ArtifactType        string
	InstanceID          string
	InstanceIP          string
	Status              string
	Location            string
	InstanceType        string
	OSVolumeID          string
	VolumeID            string
	VolumeIDs           []string
	VolumeIDsByLocation map[string]string
	SourceOSVolumeID    string
	VolumeName          string
	VolumeLocation      string
	VolumeLocations     []string
	VolumeStatus        string
	ClonedVolume        bool
	KeepInstance        bool
	DeleteConfig        DeleteConfig
	StateData           map[string]interface{}
}

func newArtifact(client verdaClient, config *Config, instance instanceState, volume *volumeArtifactState) *Artifact {
	artifact := &Artifact{
		Client:       client,
		ArtifactType: config.ArtifactType,
		InstanceID:   instance.ID,
		InstanceIP:   instance.IP,
		Status:       instance.Status,
		Location:     instance.Location,
		InstanceType: instance.InstanceType,
		OSVolumeID:   instance.OSVolumeID,
		KeepInstance: config.KeepInstance,
		DeleteConfig: DeleteConfig{
			VolumeIDs:         append([]string(nil), config.VolumeIDsToDelete...),
			DeletePermanently: config.DeletePermanently,
			DeleteOnDestroy:   !config.KeepInstance,
		},
		StateData: generatedData(instance),
	}
	if volume == nil {
		return artifact
	}

	replicas := normalizedVolumeReplicas(*volume)
	summary := summarizeVolumeReplicas(replicas)
	artifact.VolumeID = volume.ID
	artifact.VolumeIDs = summary.IDs
	artifact.VolumeIDsByLocation = summary.IDsByLocation
	artifact.SourceOSVolumeID = volume.SourceOSVolumeID
	artifact.VolumeName = volume.Name
	artifact.VolumeLocation = volume.Location
	artifact.VolumeLocations = summary.Locations
	artifact.VolumeStatus = volume.Status
	artifact.ClonedVolume = volume.Cloned
	artifact.DeleteConfig.VolumeIDs = summary.IDs
	artifact.DeleteConfig.DeleteOnDestroy = true
	artifact.StateData = generatedDataForArtifact(instance, *volume)
	return artifact
}

// BuilderId returns the Packer builder ID that produced this artifact.
//
//revive:disable-next-line:var-naming
func (a *Artifact) BuilderId() string {
	return BuilderID
}

// Files returns files associated with the artifact.
func (a *Artifact) Files() []string {
	return nil
}

// Id returns the Verda artifact ID.
//
//revive:disable-next-line:var-naming
func (a *Artifact) Id() string {
	if a.ArtifactType == artifactTypeOSVolume {
		return a.VolumeID
	}
	return a.InstanceID
}

func (a *Artifact) String() string {
	if a.ArtifactType == artifactTypeOSVolume {
		if len(a.VolumeIDs) > 1 {
			lines := []string{fmt.Sprintf("Verda OS volumes (%d):", len(a.VolumeIDs))}
			for i, id := range a.VolumeIDs {
				entry := id
				if i < len(a.VolumeLocations) {
					if location := a.VolumeLocations[i]; location != "" {
						entry = fmt.Sprintf("%s: %s", location, id)
					}
				}
				if id == a.VolumeID {
					entry += " (primary)"
				}
				lines = append(lines, "  "+entry)
			}
			if a.ClonedVolume && a.SourceOSVolumeID != "" {
				lines = append(lines, "Cloned from: "+a.SourceOSVolumeID)
			}
			return strings.Join(lines, "\n")
		}

		message := fmt.Sprintf("Verda OS volume: %s", a.VolumeID)
		if a.VolumeLocation != "" {
			message += fmt.Sprintf(" (%s)", a.VolumeLocation)
		}
		if a.ClonedVolume && a.SourceOSVolumeID != "" {
			message += fmt.Sprintf(" (cloned from %s)", a.SourceOSVolumeID)
		}
		return message
	}

	message := fmt.Sprintf("Verda instance: %s", a.InstanceID)
	if a.InstanceIP != "" {
		message += fmt.Sprintf(" (%s)", a.InstanceIP)
	}
	if a.KeepInstance {
		message += "\nInstance retained because keep_instance is true."
	}
	return message
}

// State returns provider-specific artifact state.
func (a *Artifact) State(name string) interface{} {
	return a.StateData[name]
}

// Destroy deletes the Verda resource when artifact cleanup is requested.
func (a *Artifact) Destroy() error {
	if a.Client == nil || !a.DeleteConfig.DeleteOnDestroy {
		return nil
	}
	if a.ArtifactType == artifactTypeOSVolume {
		volumeIDs := a.DeleteConfig.VolumeIDs
		if len(volumeIDs) == 0 && a.VolumeID != "" {
			volumeIDs = []string{a.VolumeID}
		}
		if len(volumeIDs) == 0 {
			return nil
		}
		var errs []error
		for _, id := range volumeIDs {
			if err := a.Client.DeleteVolume(context.Background(), id, a.DeleteConfig.DeletePermanently); err != nil {
				errs = append(errs, fmt.Errorf("deleting volume %s: %w", id, err))
			}
		}
		return errors.Join(errs...)
	}
	if a.InstanceID == "" {
		return nil
	}
	return a.Client.DeleteInstance(
		context.Background(),
		a.InstanceID,
		a.DeleteConfig.VolumeIDs,
		a.DeleteConfig.DeletePermanently,
	)
}
