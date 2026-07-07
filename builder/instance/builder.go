package instance

import (
	"context"
	"fmt"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// BuilderID identifies artifacts produced by the Verda instance builder.
const BuilderID = "verda.instance"

// Builder creates Verda instances and runs Packer provisioners against them.
type Builder struct {
	config Config
	runner multistep.Runner
}

// ConfigSpec returns the HCL2 schema for the builder.
func (b *Builder) ConfigSpec() hcldec.ObjectSpec {
	return b.config.FlatMapstructure().HCL2Spec()
}

// Prepare decodes and validates the builder configuration.
func (b *Builder) Prepare(raws ...interface{}) ([]string, []string, error) {
	return b.config.Prepare(raws...)
}

// Run executes the Verda instance build.
func (b *Builder) Run(ctx context.Context, ui packer.Ui, hook packer.Hook) (packer.Artifact, error) {
	state := new(multistep.BasicStateBag)
	state.Put("hook", hook)
	state.Put("ui", ui)

	steps := []multistep.Step{
		&stepCreateClient{Config: &b.config},
		&communicator.StepSSHKeyGen{
			CommConf:            &b.config.Comm,
			SSHTemporaryKeyPair: b.config.Comm.SSHTemporaryKeyPair,
		},
		&stepCreateSSHKey{Config: &b.config},
		&stepCreateStartupScript{Config: &b.config},
		&stepCreateInstance{Config: &b.config},
		&stepWaitForInstance{Config: &b.config},
		&communicator.StepConnect{
			Config:    &b.config.Comm,
			Host:      communicator.CommHost(b.config.Comm.SSHHost, stateKeyInstanceIP),
			SSHConfig: b.config.Comm.SSHConfigFunc(),
			SSHPort:   func(multistep.StateBag) (int, error) { return b.config.Comm.SSHPort, nil },
		},
		new(commonsteps.StepProvision),
		&commonsteps.StepCleanupTempKeys{Comm: &b.config.Comm},
		&stepCreateOSVolumeArtifact{Config: &b.config},
	}

	b.runner = commonsteps.NewRunner(steps, b.config.PackerConfig, ui)
	b.runner.Run(ctx, state)

	if err, ok := state.GetOk("error"); ok {
		return nil, err.(error)
	}

	instanceState, ok := instanceFromState(state)
	if !ok {
		return nil, fmt.Errorf("instance state is missing after build")
	}

	artifact := &Artifact{
		Client:       clientFromState(state),
		ArtifactType: b.config.ArtifactType,
		InstanceID:   instanceState.ID,
		InstanceIP:   instanceState.IP,
		Status:       instanceState.Status,
		Location:     instanceState.Location,
		InstanceType: instanceState.InstanceType,
		OSVolumeID:   instanceState.OSVolumeID,
		KeepInstance: b.config.KeepInstance,
		DeleteConfig: DeleteConfig{
			VolumeIDs:         b.config.VolumeIDsToDelete,
			DeletePermanently: b.config.DeletePermanently,
			DeleteOnDestroy:   !b.config.KeepInstance,
		},
		StateData: generatedData(instanceState),
	}
	if b.config.ArtifactType == artifactTypeOSVolume {
		volumeState, ok := volumeArtifactFromState(state)
		if !ok {
			return nil, fmt.Errorf("volume artifact state is missing after build")
		}
		artifact.VolumeID = volumeState.ID
		artifact.SourceOSVolumeID = volumeState.SourceOSVolumeID
		artifact.VolumeName = volumeState.Name
		artifact.VolumeLocation = volumeState.Location
		artifact.VolumeStatus = volumeState.Status
		artifact.ClonedVolume = volumeState.Cloned
		artifact.DeleteConfig.DeleteOnDestroy = true
		artifact.StateData = generatedDataForArtifact(instanceState, volumeState)
	}

	return artifact, nil
}
