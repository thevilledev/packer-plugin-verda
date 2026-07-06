package instance

import (
	"context"
	"fmt"
)

// DeleteConfig controls artifact destruction behavior.
type DeleteConfig struct {
	VolumeIDs         []string
	DeletePermanently bool
	DeleteOnDestroy   bool
}

// Artifact describes the Verda resource produced by a build.
type Artifact struct {
	Client           verdaClient
	ArtifactType     string
	InstanceID       string
	InstanceIP       string
	Status           string
	Location         string
	InstanceType     string
	OSVolumeID       string
	VolumeID         string
	SourceOSVolumeID string
	VolumeName       string
	VolumeLocation   string
	VolumeStatus     string
	ClonedVolume     bool
	KeepInstance     bool
	DeleteConfig     DeleteConfig
	StateData        map[string]interface{}
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
		message := fmt.Sprintf("Verda OS volume: %s", a.VolumeID)
		if a.ClonedVolume {
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

// Destroy deletes the Verda instance when artifact cleanup is requested.
func (a *Artifact) Destroy() error {
	if a.Client == nil || !a.DeleteConfig.DeleteOnDestroy {
		return nil
	}
	if a.ArtifactType == artifactTypeOSVolume {
		if a.VolumeID == "" {
			return nil
		}
		return a.Client.DeleteVolume(context.Background(), a.VolumeID, a.DeleteConfig.DeletePermanently)
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
