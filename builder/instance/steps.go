package instance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

type stepCreateClient struct {
	Config *Config
}

func (s *stepCreateClient) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client, err := newSDKClient(s.Config)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}
	putClient(state, sdkClient{client: client})
	return multistep.ActionContinue
}

func (s *stepCreateClient) Cleanup(multistep.StateBag) {}

type stepCreateSSHKey struct {
	Config *Config
}

func (s *stepCreateSSHKey) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	if s.Config.SkipTemporarySSHKey || s.Config.Comm.Type != "ssh" {
		return multistep.ActionContinue
	}
	if len(s.Config.Comm.SSHPublicKey) == 0 {
		state.Put("error", errors.New("temporary SSH public key was not generated"))
		return multistep.ActionHalt
	}

	ui := state.Get("ui").(packer.Ui)
	ui.Say("Creating temporary Verda SSH key...")

	key, err := getClient(state).CreateSSHKey(ctx, verda.CreateSSHKeyRequest{
		Name:      s.Config.TemporarySSHKeyName,
		PublicKey: strings.TrimSpace(string(s.Config.Comm.SSHPublicKey)),
	})
	if err != nil {
		state.Put("error", fmt.Errorf("creating temporary SSH key: %w", err))
		return multistep.ActionHalt
	}
	state.Put(stateKeyCreatedSSHKeyID, key.ID)
	s.Config.SSHKeyIDs = append([]string{key.ID}, s.Config.SSHKeyIDs...)
	return multistep.ActionContinue
}

func (s *stepCreateSSHKey) Cleanup(state multistep.StateBag) {
	id, ok := state.Get(stateKeyCreatedSSHKeyID).(string)
	if !ok || id == "" {
		return
	}
	ui := state.Get("ui").(packer.Ui)
	ui.Say("Deleting temporary Verda SSH key...")
	if err := getClient(state).DeleteSSHKey(context.Background(), id); err != nil {
		ui.Error(fmt.Sprintf("Error deleting temporary SSH key %s: %s", id, err))
	}
}

type stepCreateStartupScript struct {
	Config *Config
}

func (s *stepCreateStartupScript) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	if s.Config.StartupScript == "" {
		return multistep.ActionContinue
	}

	name := s.Config.StartupScriptName
	if name == "" {
		name = s.Config.Hostname + "-packer-startup"
	}

	ui := state.Get("ui").(packer.Ui)
	ui.Say("Creating Verda startup script...")

	script, err := getClient(state).CreateStartupScript(ctx, verda.CreateStartupScriptRequest{
		Name:   name,
		Script: s.Config.StartupScript,
	})
	if err != nil {
		state.Put("error", fmt.Errorf("creating startup script: %w", err))
		return multistep.ActionHalt
	}
	state.Put(stateKeyCreatedScriptID, script.ID)
	s.Config.StartupScriptID = script.ID
	return multistep.ActionContinue
}

func (s *stepCreateStartupScript) Cleanup(state multistep.StateBag) {
	if !s.Config.DeleteStartupScript {
		return
	}
	id, ok := state.Get(stateKeyCreatedScriptID).(string)
	if !ok || id == "" {
		return
	}
	ui := state.Get("ui").(packer.Ui)
	ui.Say("Deleting temporary Verda startup script...")
	if err := getClient(state).DeleteStartupScript(context.Background(), id); err != nil {
		ui.Error(fmt.Sprintf("Error deleting startup script %s: %s", id, err))
	}
}

type stepCreateInstance struct {
	Config *Config
}

func (s *stepCreateInstance) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	ui.Say("Creating Verda instance...")

	req := verda.CreateInstanceRequest{
		InstanceType:    s.Config.InstanceType,
		Image:           s.Config.Image,
		Hostname:        s.Config.Hostname,
		Description:     s.Config.Description,
		SSHKeyIDs:       s.Config.SSHKeyIDs,
		LocationCode:    s.Config.LocationCode,
		Contract:        s.Config.Contract,
		Pricing:         s.Config.Pricing,
		ExistingVolumes: s.Config.ExistingVolumeIDs,
		IsSpot:          s.Config.IsSpot,
		Volumes:         make([]verda.VolumeCreateRequest, 0, len(s.Config.Volumes)),
	}
	if s.Config.StartupScriptID != "" {
		req.StartupScriptID = &s.Config.StartupScriptID
	}
	if s.Config.Coupon != "" {
		req.Coupon = &s.Config.Coupon
	}
	if s.Config.OSVolumeName != "" || s.Config.OSVolumeSize > 0 {
		req.OSVolume = &verda.OSVolumeCreateRequest{
			Name:              firstNonEmpty(s.Config.OSVolumeName, s.Config.Hostname+"-os"),
			Size:              s.Config.OSVolumeSize,
			OnSpotDiscontinue: s.Config.OSVolumeSpotBehavior,
		}
	}
	for _, volume := range s.Config.Volumes {
		req.Volumes = append(req.Volumes, verda.VolumeCreateRequest{
			Name:              volume.Name,
			Size:              volume.Size,
			Type:              volume.Type,
			LocationCode:      firstNonEmpty(volume.LocationCode, s.Config.LocationCode),
			OnSpotDiscontinue: volume.OnSpotDiscontinue,
		})
	}

	instance, err := getClient(state).CreateInstance(ctx, req)
	if err != nil {
		state.Put("error", fmt.Errorf("creating instance: %w", err))
		return multistep.ActionHalt
	}
	current := saveInstanceState(state, instance)
	ui.Say(fmt.Sprintf("Created instance %s", current.ID))
	return multistep.ActionContinue
}

func (s *stepCreateInstance) Cleanup(state multistep.StateBag) {
	current, ok := instanceFromState(state)
	if !ok || current.ID == "" || s.Config.KeepInstance {
		return
	}
	ui := state.Get("ui").(packer.Ui)
	ui.Say("Deleting Verda instance...")
	volumeIDsToDelete := s.Config.VolumeIDsToDelete
	if shouldPreserveSourceOSVolume(s.Config, state, current) {
		volumeIDsToDelete = []string{}
	}
	if err := getClient(state).DeleteInstance(context.Background(), current.ID, volumeIDsToDelete, s.Config.DeletePermanently); err != nil {
		ui.Error(fmt.Sprintf("Error deleting instance %s: %s", current.ID, err))
	}
}

type stepWaitForInstance struct {
	Config *Config
}

func (s *stepWaitForInstance) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	current, ok := instanceFromState(state)
	if !ok {
		state.Put("error", errors.New("instance state is missing"))
		return multistep.ActionHalt
	}

	ui := state.Get("ui").(packer.Ui)
	deadline := time.NewTimer(s.Config.InstanceTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(s.Config.PollInterval)
	defer ticker.Stop()

	ui.Say("Waiting for Verda instance to become reachable...")
	for {
		instance, err := getClient(state).GetInstance(ctx, current.ID)
		if err != nil {
			state.Put("error", fmt.Errorf("waiting for instance %s: %w", current.ID, err))
			return multistep.ActionHalt
		}
		current = saveInstanceState(state, instance)
		if current.IP != "" && statusAllowed(current.Status, s.Config.AllowedSSHStatuses) {
			ui.Say(fmt.Sprintf("Instance %s is %s at %s", current.ID, current.Status, current.IP))
			return multistep.ActionContinue
		}
		if isTerminalStatus(current.Status) {
			state.Put("error", fmt.Errorf("instance %s reached terminal status %q", current.ID, current.Status))
			return multistep.ActionHalt
		}

		select {
		case <-ctx.Done():
			state.Put("error", ctx.Err())
			return multistep.ActionHalt
		case <-deadline.C:
			state.Put("error", fmt.Errorf("timeout waiting for instance %s to become reachable", current.ID))
			return multistep.ActionHalt
		case <-ticker.C:
		}
	}
}

func (s *stepWaitForInstance) Cleanup(multistep.StateBag) {}

type stepCreateOSVolumeArtifact struct {
	Config *Config
}

func (s *stepCreateOSVolumeArtifact) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	if s.Config.ArtifactType != artifactTypeOSVolume {
		return multistep.ActionContinue
	}

	current, ok := instanceFromState(state)
	if !ok || current.ID == "" {
		state.Put("error", errors.New("instance state is missing"))
		return multistep.ActionHalt
	}
	if current.OSVolumeID == "" {
		state.Put("error", fmt.Errorf("instance %s does not expose an OS volume ID", current.ID))
		return multistep.ActionHalt
	}

	ui := state.Get("ui").(packer.Ui)
	client := getClient(state)
	if !s.Config.SkipShutdownBeforeArtifact {
		ui.Say("Shutting down Verda instance before capturing OS volume...")
		if err := client.ShutdownInstance(ctx, current.ID); err != nil {
			state.Put("error", fmt.Errorf("shutting down instance %s: %w", current.ID, err))
			return multistep.ActionHalt
		}
		if _, err := waitForInstanceStatuses(ctx, client, current.ID, s.Config, verda.StatusOffline); err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}
	}

	volume := volumeArtifactState{
		ID:               current.OSVolumeID,
		SourceOSVolumeID: current.OSVolumeID,
		Location:         current.Location,
		Cloned:           false,
	}
	if s.Config.shouldCloneOSVolume() {
		name := s.Config.ArtifactVolumeName
		if name == "" {
			name = current.ID + "-packer-os-volume"
		}
		ui.Say("Cloning Verda OS volume for artifact...")
		clonedID, err := client.CloneVolume(ctx, current.OSVolumeID, verda.VolumeCloneRequest{
			Name:         name,
			LocationCode: s.Config.ArtifactVolumeLocationCode,
		})
		if err != nil {
			state.Put("error", fmt.Errorf("cloning OS volume %s: %w", current.OSVolumeID, err))
			return multistep.ActionHalt
		}
		volume.ID = clonedID
		volume.Name = name
		volume.Location = s.Config.ArtifactVolumeLocationCode
		volume.Cloned = true
		ui.Say(fmt.Sprintf("Created OS volume artifact %s", clonedID))
	} else {
		ui.Say(fmt.Sprintf("Using source OS volume %s as artifact", current.OSVolumeID))
	}

	if sdkVolume, err := waitForVolumeReady(ctx, client, volume.ID, s.Config); err == nil {
		volume.Name = firstNonEmpty(volume.Name, sdkVolume.Name)
		volume.Location = firstNonEmpty(volume.Location, sdkVolume.Location)
		volume.Status = sdkVolume.Status
	} else if volume.Cloned {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	saveVolumeArtifactState(state, volume)
	return multistep.ActionContinue
}

func (s *stepCreateOSVolumeArtifact) Cleanup(multistep.StateBag) {}

func shouldPreserveSourceOSVolume(config *Config, state multistep.StateBag, current instanceState) bool {
	if config.ArtifactType != artifactTypeOSVolume || config.shouldCloneOSVolume() {
		return false
	}
	volume, ok := volumeArtifactFromState(state)
	return ok && volume.ID == current.OSVolumeID
}

func waitForInstanceStatuses(ctx context.Context, client verdaClient, id string, config *Config, statuses ...string) (*verda.Instance, error) {
	deadline := time.NewTimer(config.InstanceTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	for {
		instance, err := client.GetInstance(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("waiting for instance %s: %w", id, err)
		}
		for _, status := range statuses {
			if strings.EqualFold(instance.Status, status) {
				return instance, nil
			}
		}
		if isTerminalStatus(instance.Status) && !statusAllowed(instance.Status, statuses) {
			return nil, fmt.Errorf("instance %s reached terminal status %q", id, instance.Status)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, fmt.Errorf("timeout waiting for instance %s to reach %s", id, strings.Join(statuses, ", "))
		case <-ticker.C:
		}
	}
}

func waitForVolumeReady(ctx context.Context, client verdaClient, id string, config *Config) (*verda.Volume, error) {
	deadline := time.NewTimer(config.InstanceTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	for {
		volume, err := client.GetVolume(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("waiting for volume %s: %w", id, err)
		}
		switch strings.ToLower(volume.Status) {
		case verda.VolumeStatusCreated, verda.VolumeStatusDetached, verda.VolumeStatusAttached:
			return volume, nil
		case verda.VolumeStatusDeleted, verda.VolumeStatusDeleting, verda.VolumeStatusCanceled:
			return nil, fmt.Errorf("volume %s reached terminal status %q", id, volume.Status)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, fmt.Errorf("timeout waiting for volume %s to become ready", id)
		case <-ticker.C:
		}
	}
}

func statusAllowed(status string, allowed []string) bool {
	for _, candidate := range allowed {
		if strings.EqualFold(status, candidate) {
			return true
		}
	}
	return false
}

func isTerminalStatus(status string) bool {
	switch strings.ToLower(status) {
	case "error", "notfound", "deleting", "deleted", "discontinued", "no_capacity":
		return true
	default:
		return false
	}
}
