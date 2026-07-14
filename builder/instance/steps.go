package instance

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

type stepCreateClient struct {
	Config        *Config
	ClientFactory verdaClientFactory
}

func (s *stepCreateClient) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	factory := s.ClientFactory
	if factory == nil {
		factory = newVerdaClient
	}
	client, err := factory(s.Config)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}
	putClient(state, client)
	return multistep.ActionContinue
}

func (s *stepCreateClient) Cleanup(multistep.StateBag) {}

type stepCreateSSHKey struct {
	Config *Config
}

func (s *stepCreateSSHKey) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	if s.Config.SkipTemporarySSHKey {
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

	req := s.Config.instanceRequest(
		createdSSHKeyIDFromState(state),
		createdStartupScriptIDFromState(state),
	)

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
	if s.Config.ArtifactType == artifactTypeInstance && buildComplete(state) {
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
	requiresIP := s.Config.Comm.Type == "ssh"
	ui.Say("Waiting for Verda instance to become ready...")
	ready, err := pollUntil(
		ctx,
		s.Config.InstanceTimeout,
		s.Config.PollInterval,
		func(ctx context.Context) (instanceState, bool, error) {
			instance, err := getClient(state).GetInstance(ctx, current.ID)
			if err != nil {
				return instanceState{}, false, fmt.Errorf("waiting for instance %s: %w", current.ID, err)
			}
			latest := saveInstanceState(state, instance)
			if isTerminalStatus(latest.Status) {
				return instanceState{}, false, fmt.Errorf("instance %s reached terminal status %q", latest.ID, latest.Status)
			}
			statusReady := statusAllowed(latest.Status, s.Config.AllowedSSHStatuses)
			return latest, statusReady && (!requiresIP || latest.IP != ""), nil
		},
		func() error {
			return fmt.Errorf("timeout waiting for instance %s to become ready", current.ID)
		},
	)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}
	message := fmt.Sprintf("Instance %s is %s", ready.ID, ready.Status)
	if ready.IP != "" {
		message += " at " + ready.IP
	}
	ui.Say(message)
	return multistep.ActionContinue
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
		locations = artifactVolumeLocationsWithSource(locations, sourceLocation)
		sourceType := firstNonEmpty(sourceVolume.Type, verda.VolumeTypeNVMe)
		// Verda rejects cross-datacenter clones of an OS volume that is still
		// attached to an instance. Create and detach the local artifact first,
		// then use it as the source for the remaining locations.
		ui.Say(fmt.Sprintf("Cloning Verda OS volume for artifact in %s...", sourceLocation))
		seedID, err := client.CloneVolume(ctx, current.OSVolumeID, volumeCloneRequest{
			Name:         name,
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
			Name:     name,
			Location: sourceLocation,
			Cloned:   true,
		}
		ui.Say(fmt.Sprintf("Created OS volume artifact %s in %s", seedID, sourceLocation))
		sdkVolume, err := waitForClonedVolumeStatuses(ctx, client, seedID, s.Config, verda.VolumeStatusDetached)
		if err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}
		seed.Name = firstNonEmpty(sdkVolume.Name, seed.Name)
		seed.Location = firstNonEmpty(sdkVolume.Location, seed.Location)
		seed.Status = sdkVolume.Status

		replicas := make([]volumeReplicaState, 0, len(locations))
		for _, location := range locations {
			if strings.EqualFold(location, sourceLocation) {
				replicas = append(replicas, seed)
				continue
			}

			ui.Say(fmt.Sprintf("Cloning Verda OS volume for artifact in %s...", location))
			clonedID, err := client.CloneVolume(ctx, seed.ID, volumeCloneRequest{
				Name:         name,
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
				Name:     name,
				Location: location,
				Cloned:   true,
			})
			ui.Say(fmt.Sprintf("Created OS volume artifact %s in %s", clonedID, location))
		}

		for i := range replicas {
			if replicas[i].ID == seed.ID {
				continue
			}
			sdkVolume, err := waitForClonedVolumeReady(ctx, client, replicas[i].ID, s.Config)
			if err != nil {
				state.Put("error", err)
				return multistep.ActionHalt
			}
			replicas[i].Name = firstNonEmpty(sdkVolume.Name, replicas[i].Name)
			replicas[i].Location = firstNonEmpty(sdkVolume.Location, replicas[i].Location)
			replicas[i].Status = sdkVolume.Status
		}

		volume.ID = replicas[0].ID
		volume.Name = replicas[0].Name
		volume.Location = replicas[0].Location
		volume.Status = replicas[0].Status
		volume.Cloned = true
		volume.Replicas = replicas
	} else {
		ui.Say(fmt.Sprintf("Using source OS volume %s as artifact", current.OSVolumeID))
		sdkVolume, err := waitForVolumeReady(ctx, client, volume.ID, s.Config)
		if err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}
		volume.Name = firstNonEmpty(volume.Name, sdkVolume.Name)
		volume.Location = firstNonEmpty(volume.Location, sdkVolume.Location)
		volume.Status = sdkVolume.Status
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
	if buildComplete(state) {
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
		if err := deletePartialVolumeArtifact(context.Background(), client, id, s.Config); err != nil {
			ui.Error(fmt.Sprintf("Error deleting partial OS volume artifact %s: %s", id, err))
		}
	}
}

func rememberCreatedArtifactVolume(state multistep.StateBag, id string) {
	ids, _ := state.Get(stateKeyCreatedArtifactVolumeIDs).([]string)
	state.Put(stateKeyCreatedArtifactVolumeIDs, append(ids, id))
}

func deletePartialVolumeArtifact(ctx context.Context, client verdaClient, id string, config *Config) error {
	deletePermanently := false
	if config != nil {
		deletePermanently = config.DeletePermanently
	}
	_, err := pollUntil(
		ctx,
		effectiveInstanceTimeout(config),
		effectivePollInterval(config),
		func(ctx context.Context) (struct{}, bool, error) {
			if err := client.DeleteVolume(ctx, id, deletePermanently); err != nil {
				if isNotFound(err) {
					return struct{}{}, true, nil
				}
				if isVolumeCloneInProgress(err) {
					return struct{}{}, false, nil
				}
				return struct{}{}, false, err
			}
			return struct{}{}, true, nil
		},
		func() error {
			return fmt.Errorf("timeout deleting volume %s while clone operation remained in progress", id)
		},
	)
	return err
}

func shouldPreserveSourceOSVolume(config *Config, state multistep.StateBag, current instanceState) bool {
	if !buildComplete(state) || config.ArtifactType != artifactTypeOSVolume || config.shouldCloneOSVolume() {
		return false
	}
	volume, ok := volumeArtifactFromState(state)
	return ok && volume.ID == current.OSVolumeID
}

type stepMarkBuildComplete struct{}

func (s *stepMarkBuildComplete) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	if err := ctx.Err(); err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}
	state.Put(stateKeyBuildComplete, true)
	return multistep.ActionContinue
}

func (s *stepMarkBuildComplete) Cleanup(multistep.StateBag) {}

func artifactVolumeLocations(config *Config, fallback string) []string {
	if len(config.ArtifactVolumeLocationCodes) > 0 {
		return config.ArtifactVolumeLocationCodes
	}
	if config.ArtifactVolumeLocationCode != "" {
		return []string{config.ArtifactVolumeLocationCode}
	}
	return []string{firstNonEmpty(fallback, config.LocationCode, defaultLocationCode)}
}

func artifactVolumeLocationsWithSource(locations []string, sourceLocation string) []string {
	for _, candidate := range locations {
		if strings.EqualFold(candidate, sourceLocation) {
			return locations
		}
	}
	return append(append([]string{}, locations...), sourceLocation)
}

func waitForInstanceStatuses(ctx context.Context, client verdaClient, id string, config *Config, statuses ...string) (*verda.Instance, error) {
	return pollUntil(
		ctx,
		config.InstanceTimeout,
		config.PollInterval,
		func(ctx context.Context) (*verda.Instance, bool, error) {
			instance, err := client.GetInstance(ctx, id)
			if err != nil {
				return nil, false, fmt.Errorf("waiting for instance %s: %w", id, err)
			}
			if statusAllowed(instance.Status, statuses) {
				return instance, true, nil
			}
			if isTerminalStatus(instance.Status) {
				return nil, false, fmt.Errorf("instance %s reached terminal status %q", id, instance.Status)
			}
			return nil, false, nil
		},
		func() error {
			return fmt.Errorf("timeout waiting for instance %s to reach %s", id, strings.Join(statuses, ", "))
		},
	)
}

func waitForVolumeReady(ctx context.Context, client verdaClient, id string, config *Config) (*verda.Volume, error) {
	return waitForVolumeStatuses(
		ctx,
		client,
		id,
		config,
		false,
		verda.VolumeStatusCreated,
		verda.VolumeStatusDetached,
		verda.VolumeStatusAttached,
	)
}

func waitForClonedVolumeReady(ctx context.Context, client verdaClient, id string, config *Config) (*verda.Volume, error) {
	return waitForClonedVolumeStatuses(
		ctx,
		client,
		id,
		config,
		verda.VolumeStatusCreated,
		verda.VolumeStatusDetached,
		verda.VolumeStatusAttached,
	)
}

func waitForClonedVolumeStatuses(
	ctx context.Context,
	client verdaClient,
	id string,
	config *Config,
	statuses ...string,
) (*verda.Volume, error) {
	return waitForVolumeStatuses(ctx, client, id, config, true, statuses...)
}

func waitForVolumeStatuses(
	ctx context.Context,
	client verdaClient,
	id string,
	config *Config,
	retryNotFound bool,
	statuses ...string,
) (*verda.Volume, error) {
	return pollUntil(
		ctx,
		config.InstanceTimeout,
		config.PollInterval,
		func(ctx context.Context) (*verda.Volume, bool, error) {
			volume, err := client.GetVolume(ctx, id)
			if err != nil {
				if retryNotFound && isNotFound(err) {
					return nil, false, nil
				}
				return nil, false, fmt.Errorf("waiting for volume %s: %w", id, err)
			}
			if statusAllowed(volume.Status, statuses) {
				return volume, true, nil
			}
			switch strings.ToLower(volume.Status) {
			case verda.VolumeStatusDeleted, verda.VolumeStatusDeleting, verda.VolumeStatusCanceled:
				return nil, false, fmt.Errorf("volume %s reached terminal status %q", id, volume.Status)
			}
			return nil, false, nil
		},
		func() error {
			return fmt.Errorf("timeout waiting for volume %s to reach %s", id, strings.Join(statuses, ", "))
		},
	)
}

func effectiveInstanceTimeout(config *Config) time.Duration {
	if config != nil && config.InstanceTimeout > 0 {
		return config.InstanceTimeout
	}
	return defaultInstanceTimeout
}

func effectivePollInterval(config *Config) time.Duration {
	if config != nil && config.PollInterval > 0 {
		return config.PollInterval
	}
	return defaultPollInterval
}

func isNotFound(err error) bool {
	var apiErr *verda.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

func isVolumeCloneInProgress(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *verda.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusBadRequest {
		message := strings.ToLower(apiErr.Code + " " + apiErr.Message + " " + apiErr.Details)
		if strings.Contains(message, "cloning") {
			return true
		}
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "middle of cloning process") ||
		(strings.Contains(message, "cloning") && strings.Contains(message, "cannot be deleted"))
}

func pollUntil[T any](
	ctx context.Context,
	timeout time.Duration,
	interval time.Duration,
	check func(context.Context) (T, bool, error),
	timeoutError func() error,
) (T, error) {
	var zero T
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		value, done, err := check(ctx)
		if err != nil {
			return zero, err
		}
		if done {
			return value, nil
		}
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-deadline.C:
			return zero, timeoutError()
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
	case verda.StatusError, verda.StatusNotFound, verda.StatusDeleting, "deleted", verda.StatusDiscontinued, verda.StatusNoCapacity:
		return true
	default:
		return false
	}
}
