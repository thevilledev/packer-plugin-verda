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
		Name: s.Config.TemporarySSHKeyName,
		PublicKey: temporarySSHPublicKey(
			s.Config.Comm.SSHPublicKey,
			firstNonEmpty(s.Config.Comm.SSHTemporaryKeyPairName, s.Config.TemporarySSHKeyName),
		),
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

func temporarySSHPublicKey(publicKey []byte, comment string) string {
	trimmed := strings.TrimSpace(string(publicKey))
	if comment == "" {
		return trimmed
	}
	parts := strings.Fields(trimmed)
	if len(parts) < 2 {
		return trimmed
	}
	return parts[0] + " " + parts[1] + " " + comment
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
		sourceVolume, err := client.GetVolume(ctx, current.OSVolumeID)
		if err != nil {
			state.Put("error", fmt.Errorf("getting source OS volume %s: %w", current.OSVolumeID, err))
			return multistep.ActionHalt
		}

		name := s.Config.ArtifactVolumeName
		if name == "" {
			name = current.ID + "-packer-os-volume"
		}

		locations := artifactVolumeLocations(s.Config, current.Location)
		sourceLocation := firstNonEmpty(sourceVolume.Location, current.Location, s.Config.LocationCode, defaultLocationCode)
		sourceType := firstNonEmpty(sourceVolume.Type, verda.VolumeTypeNVMe)
		seedName := artifactVolumeSeedName(
			name,
			sourceLocation,
			artifactVolumeLocationRequested(locations, sourceLocation),
			len(locations) > 1,
		)
		ui.Say(fmt.Sprintf("Cloning Verda OS volume for artifact seed in %s...", sourceLocation))
		seedID, err := client.CloneVolume(ctx, current.OSVolumeID, volumeCloneRequest{
			Name:         seedName,
			LocationCode: sourceLocation,
			Type:         sourceType,
		})
		if err != nil {
			state.Put("error", fmt.Errorf("cloning OS volume %s to %s: %w", current.OSVolumeID, sourceLocation, err))
			return multistep.ActionHalt
		}
		rememberCreatedArtifactVolume(state, seedID)
		seed := volumeReplicaState{
			ID:       seedID,
			Name:     seedName,
			Location: sourceLocation,
			Cloned:   true,
		}
		ui.Say(fmt.Sprintf("Created OS volume artifact seed %s in %s", seedID, sourceLocation))
		sdkVolume, err := waitForVolumeReady(ctx, client, seedID, s.Config)
		if err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}
		seed.Name = firstNonEmpty(sdkVolume.Name, seed.Name)
		seed.Location = firstNonEmpty(sdkVolume.Location, seed.Location)
		seed.Status = sdkVolume.Status

		replicas := make([]volumeReplicaState, 0, len(locations))
		seedIsArtifact := false
		for _, location := range locations {
			cloneName := artifactVolumeName(name, location, len(locations) > 1)
			if strings.EqualFold(location, sourceLocation) {
				seed.Name = firstNonEmpty(seed.Name, cloneName)
				replicas = append(replicas, seed)
				seedIsArtifact = true
				continue
			}

			ui.Say(fmt.Sprintf("Cloning Verda OS volume for artifact in %s...", location))
			clonedID, err := client.CloneVolume(ctx, seed.ID, volumeCloneRequest{
				Name:         cloneName,
				LocationCode: location,
				Type:         sourceType,
			})
			if err != nil {
				state.Put("error", fmt.Errorf("cloning OS volume %s to %s: %w", seed.ID, location, err))
				return multistep.ActionHalt
			}
			rememberCreatedArtifactVolume(state, clonedID)
			replicas = append(replicas, volumeReplicaState{
				ID:       clonedID,
				Name:     cloneName,
				Location: location,
				Cloned:   true,
			})
			ui.Say(fmt.Sprintf("Created OS volume artifact %s in %s", clonedID, location))
		}

		for i := range replicas {
			if replicas[i].ID == seed.ID {
				continue
			}
			sdkVolume, err := waitForVolumeReady(ctx, client, replicas[i].ID, s.Config)
			if err != nil {
				state.Put("error", err)
				return multistep.ActionHalt
			}
			replicas[i].Name = firstNonEmpty(sdkVolume.Name, replicas[i].Name)
			replicas[i].Location = firstNonEmpty(sdkVolume.Location, replicas[i].Location)
			replicas[i].Status = sdkVolume.Status
		}

		if !seedIsArtifact {
			ui.Say(fmt.Sprintf("Deleting temporary Verda OS volume artifact seed %s...", seed.ID))
			if err := client.DeleteVolume(ctx, seed.ID, s.Config.DeletePermanently); err != nil {
				state.Put("error", fmt.Errorf("deleting temporary OS volume artifact seed %s: %w", seed.ID, err))
				return multistep.ActionHalt
			}
			forgetCreatedArtifactVolume(state, seed.ID)
		}

		volume.ID = replicas[0].ID
		volume.Name = replicas[0].Name
		volume.Location = replicas[0].Location
		volume.Status = replicas[0].Status
		volume.Cloned = true
		volume.Replicas = replicas
	} else {
		ui.Say(fmt.Sprintf("Using source OS volume %s as artifact", current.OSVolumeID))
		if sdkVolume, err := waitForVolumeReady(ctx, client, volume.ID, s.Config); err == nil {
			volume.Name = firstNonEmpty(volume.Name, sdkVolume.Name)
			volume.Location = firstNonEmpty(volume.Location, sdkVolume.Location)
			volume.Status = sdkVolume.Status
		}
		volume.Replicas = []volumeReplicaState{{
			ID:       volume.ID,
			Name:     volume.Name,
			Location: volume.Location,
			Status:   volume.Status,
			Cloned:   false,
		}}
	}

	saveVolumeArtifactState(state, volume)
	return multistep.ActionContinue
}

func (s *stepCreateOSVolumeArtifact) Cleanup(state multistep.StateBag) {
	if s.Config.ArtifactType != artifactTypeOSVolume || !s.Config.shouldCloneOSVolume() {
		return
	}
	if _, ok := volumeArtifactFromState(state); ok {
		return
	}

	ids, _ := state.Get(stateKeyCreatedArtifactVolumeIDs).([]string)
	if len(ids) == 0 {
		return
	}
	client := getClient(state)
	if client == nil {
		return
	}

	ui := state.Get("ui").(packer.Ui)
	for _, id := range ids {
		ui.Say(fmt.Sprintf("Deleting partial Verda OS volume artifact %s...", id))
		if err := client.DeleteVolume(context.Background(), id, s.Config.DeletePermanently); err != nil {
			ui.Error(fmt.Sprintf("Error deleting partial OS volume artifact %s: %s", id, err))
		}
	}
}

func rememberCreatedArtifactVolume(state multistep.StateBag, id string) {
	ids, _ := state.Get(stateKeyCreatedArtifactVolumeIDs).([]string)
	state.Put(stateKeyCreatedArtifactVolumeIDs, append(ids, id))
}

func forgetCreatedArtifactVolume(state multistep.StateBag, id string) {
	ids, _ := state.Get(stateKeyCreatedArtifactVolumeIDs).([]string)
	filtered := ids[:0]
	for _, candidate := range ids {
		if candidate != id {
			filtered = append(filtered, candidate)
		}
	}
	state.Put(stateKeyCreatedArtifactVolumeIDs, filtered)
}

func shouldPreserveSourceOSVolume(config *Config, state multistep.StateBag, current instanceState) bool {
	if config.ArtifactType != artifactTypeOSVolume || config.shouldCloneOSVolume() {
		return false
	}
	volume, ok := volumeArtifactFromState(state)
	return ok && volume.ID == current.OSVolumeID
}

func artifactVolumeLocations(config *Config, fallback string) []string {
	if len(config.ArtifactVolumeLocationCodes) > 0 {
		return config.ArtifactVolumeLocationCodes
	}
	if config.ArtifactVolumeLocationCode != "" {
		return []string{config.ArtifactVolumeLocationCode}
	}
	return []string{firstNonEmpty(fallback, config.LocationCode, defaultLocationCode)}
}

func artifactVolumeName(baseName, location string, multiLocation bool) string {
	if !multiLocation {
		return baseName
	}
	return fmt.Sprintf("%s-%s", baseName, strings.ToLower(location))
}

func artifactVolumeSeedName(baseName, sourceLocation string, seedIsArtifact, multiLocation bool) string {
	if seedIsArtifact {
		return artifactVolumeName(baseName, sourceLocation, multiLocation)
	}
	return fmt.Sprintf("%s-%s-seed", baseName, strings.ToLower(sourceLocation))
}

func artifactVolumeLocationRequested(locations []string, location string) bool {
	for _, candidate := range locations {
		if strings.EqualFold(candidate, location) {
			return true
		}
	}
	return false
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
