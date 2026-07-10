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
	config        Config
	clientFactory verdaClientFactory
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
	runConfig := b.config
	state := new(multistep.BasicStateBag)
	state.Put("hook", hook)
	state.Put("ui", ui)

	steps := []multistep.Step{
		&stepCreateClient{Config: &runConfig, ClientFactory: b.clientFactory},
		&communicator.StepSSHKeyGen{
			CommConf:            &runConfig.Comm,
			SSHTemporaryKeyPair: runConfig.Comm.SSHTemporaryKeyPair,
		},
		&stepCreateSSHKey{Config: &runConfig},
		&stepCreateStartupScript{Config: &runConfig},
		&stepCreateInstance{Config: &runConfig},
		&stepWaitForInstance{Config: &runConfig},
		&communicator.StepConnect{
			Config:    &runConfig.Comm,
			Host:      communicator.CommHost(runConfig.Comm.SSHHost, stateKeyInstanceIP),
			SSHConfig: runConfig.Comm.SSHConfigFunc(),
			SSHPort:   func(multistep.StateBag) (int, error) { return runConfig.Comm.SSHPort, nil },
		},
		new(commonsteps.StepProvision),
		&commonsteps.StepCleanupTempKeys{Comm: &runConfig.Comm},
		&stepCreateOSVolumeArtifact{Config: &runConfig},
		new(stepMarkBuildComplete),
	}

	runner := commonsteps.NewRunner(steps, runConfig.PackerConfig, ui)
	runner.Run(ctx, state)

	if value, ok := state.GetOk("error"); ok {
		if err, ok := value.(error); ok {
			return nil, err
		}
		return nil, fmt.Errorf("build failed: %v", value)
	}
	if !buildComplete(state) {
		if _, cancelled := state.GetOk(multistep.StateCancelled); cancelled {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return nil, context.Canceled
		}
		return nil, fmt.Errorf("build stopped before completion")
	}

	instanceState, ok := instanceFromState(state)
	if !ok {
		return nil, fmt.Errorf("instance state is missing after build")
	}

	var volumeState *volumeArtifactState
	if runConfig.ArtifactType == artifactTypeOSVolume {
		volume, ok := volumeArtifactFromState(state)
		if !ok {
			return nil, fmt.Errorf("volume artifact state is missing after build")
		}
		volumeState = &volume
	}

	return newArtifact(getClient(state), &runConfig, instanceState, volumeState), nil
}
