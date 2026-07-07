//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config,Volume

package instance

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/hashicorp/packer-plugin-sdk/uuid"
)

const (
	defaultAPITimeout      = 10 * time.Minute
	defaultInstanceTimeout = 30 * time.Minute
	defaultPollInterval    = 15 * time.Second
	defaultLocationCode    = "FIN-03"
	defaultSSHUsername     = "root"

	artifactTypeInstance = "instance"
	artifactTypeOSVolume = "os_volume"
)

// Config defines the verda.instance builder configuration.
type Config struct {
	common.PackerConfig `mapstructure:",squash"`
	Comm                communicator.Config `mapstructure:",squash"`
	ctx                 interpolate.Context

	// Verda client ID. It can also be set with VERDA_CLIENT_ID.
	ClientID string `mapstructure:"client_id" required:"true"`
	// Verda client secret. It can also be set with VERDA_CLIENT_SECRET.
	ClientSecret string `mapstructure:"client_secret" required:"true"`
	// Verda API base URL. Leave unset for the production API.
	BaseURL string `mapstructure:"base_url" required:"false"`
	// Enable verbose Verda SDK logging.
	Debug bool `mapstructure:"debug" required:"false"`

	// Verda instance type to create.
	InstanceType string `mapstructure:"instance_type" required:"true"`
	// Image name, image ID, or OS volume ID to boot from.
	Image string `mapstructure:"image" required:"true"`
	// Hostname for the created instance.
	Hostname string `mapstructure:"hostname" required:"true"`
	// Description for the created instance. Defaults to a Packer build description based on the hostname.
	Description string `mapstructure:"description" required:"false"`
	// Verda location code for the instance. Defaults to FIN-03.
	LocationCode string `mapstructure:"location_code" required:"false"`
	// Instance contract. Defaults to PAY_AS_YOU_GO, or SPOT when is_spot is true.
	Contract string `mapstructure:"contract" required:"false"`
	// Optional pricing value passed to the Verda API.
	Pricing string `mapstructure:"pricing" required:"false"`
	// Request a spot instance. When true, the default contract is SPOT.
	IsSpot bool `mapstructure:"is_spot" required:"false"`
	// Optional coupon value passed to the Verda API.
	Coupon string `mapstructure:"coupon" required:"false"`

	// Existing Verda SSH key IDs to add to the instance.
	SSHKeyIDs []string `mapstructure:"ssh_key_ids" required:"false"`
	// Name for the temporary Verda SSH key created from Packer's generated public key.
	TemporarySSHKeyName string `mapstructure:"temporary_ssh_key_name" required:"false"`
	// Disable temporary Verda SSH key creation. When true with SSH, provide ssh_key_ids and a matching SSH credential.
	SkipTemporarySSHKey bool `mapstructure:"skip_temporary_ssh_key" required:"false"`
	// Existing Verda startup script ID to attach to the instance.
	StartupScriptID string `mapstructure:"startup_script_id" required:"false"`
	// Startup script content to create before launching the instance.
	StartupScript string `mapstructure:"startup_script" required:"false"`
	// Name for a startup script created from startup_script.
	StartupScriptName string `mapstructure:"startup_script_name" required:"false"`
	// Delete a startup script created from startup_script during cleanup.
	DeleteStartupScript bool `mapstructure:"delete_startup_script" required:"false"`
	// Existing non-OS volume IDs to attach to the instance.
	ExistingVolumeIDs []string `mapstructure:"existing_volume_ids" required:"false"`
	// Name for the instance OS volume.
	OSVolumeName string `mapstructure:"os_volume_name" required:"false"`
	// Size, in GiB, for the instance OS volume.
	OSVolumeSize int `mapstructure:"os_volume_size" required:"false"`
	// Spot discontinuation behavior for the instance OS volume.
	OSVolumeSpotBehavior string `mapstructure:"os_volume_spot_behavior" required:"false"`
	// Additional data volumes to create with the instance.
	Volumes []Volume `mapstructure:"volume" required:"false"`

	// Artifact to return from the build. Valid values are instance and os_volume. Defaults to instance.
	ArtifactType string `mapstructure:"artifact_type" required:"false"`
	// Clone the source OS volume when artifact_type is os_volume. Defaults to true.
	CloneOSVolume *bool `mapstructure:"clone_os_volume" required:"false"`
	// Name for the cloned OS volume artifact.
	ArtifactVolumeName string `mapstructure:"artifact_volume_name" required:"false"`
	// Location for a single cloned OS volume artifact. Defaults to location_code.
	ArtifactVolumeLocationCode string `mapstructure:"artifact_volume_location_code" required:"false"`
	// Locations for cloned OS volume artifacts. The first location becomes the primary artifact ID.
	ArtifactVolumeLocationCodes []string `mapstructure:"artifact_volume_location_codes" required:"false"`
	// Skip shutting down the instance before creating an OS volume artifact.
	SkipShutdownBeforeArtifact bool `mapstructure:"skip_shutdown_before_artifact" required:"false"`

	// Keep the created instance after the build. By default the instance is deleted during cleanup.
	KeepInstance bool `mapstructure:"keep_instance" required:"false"`
	// Permanently delete resources during cleanup instead of moving them to a recoverable state.
	DeletePermanently bool `mapstructure:"delete_permanently" required:"false"`
	// Volume IDs to delete when the instance is deleted.
	VolumeIDsToDelete []string `mapstructure:"volume_ids_to_delete" required:"false"`
	// Interval between Verda instance status checks. Defaults to 15s.
	PollInterval time.Duration `mapstructure:"poll_interval" required:"false"`
	// Timeout for the instance to become reachable. Defaults to 30m.
	InstanceTimeout time.Duration `mapstructure:"instance_timeout" required:"false"`
	// HTTP client timeout for Verda API calls. Defaults to 10m.
	APITimeout time.Duration `mapstructure:"api_timeout" required:"false"`
	// Instance statuses that are acceptable for SSH connection attempts. Defaults to running.
	AllowedSSHStatuses []string `mapstructure:"allowed_ssh_statuses" required:"false"`
}

// Volume describes an additional data volume to create with the instance.
type Volume struct {
	// Name for the additional volume.
	Name string `mapstructure:"name" required:"true"`
	// Size, in GiB, for the additional volume.
	Size int `mapstructure:"size" required:"true"`
	// Volume type.
	Type string `mapstructure:"type" required:"true"`
	// Verda location code for the additional volume. Defaults to the instance location when unset.
	LocationCode string `mapstructure:"location_code" required:"false"`
	// Spot discontinuation behavior for the additional volume.
	OnSpotDiscontinue string `mapstructure:"on_spot_discontinue" required:"false"`
}

// Prepare decodes and validates builder configuration.
func (c *Config) Prepare(raws ...interface{}) ([]string, []string, error) {
	var md config.DecodeOpts
	md.PluginType = BuilderID
	md.Interpolate = true
	md.InterpolateContext = &c.ctx

	if err := config.Decode(c, &md, raws...); err != nil {
		return nil, nil, err
	}

	c.setDefaults()

	var errs []error
	errs = append(errs, c.Comm.Prepare(&c.ctx)...)
	if err := c.validate(); err != nil {
		errs = append(errs, err)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, nil, err
	}

	generated := []string{
		"ID",
		"ArtifactType",
		"InstanceID",
		"InstanceIP",
		"InstanceStatus",
		"InstanceType",
		"Location",
		"OSVolumeID",
		"SourceOSVolumeID",
		"VolumeCloned",
		"VolumeID",
		"VolumeIDs",
		"VolumeIDsByLocation",
		"VolumeLocation",
		"VolumeLocations",
		"VolumeName",
		"VolumeNamesByLocation",
		"VolumeStatus",
		"VolumeStatusesByLocation",
	}
	return generated, nil, nil
}

func (c *Config) setDefaults() {
	c.ClientID = firstNonEmpty(c.ClientID, os.Getenv("VERDA_CLIENT_ID"))
	c.ClientSecret = firstNonEmpty(c.ClientSecret, os.Getenv("VERDA_CLIENT_SECRET"))

	if c.LocationCode == "" {
		c.LocationCode = defaultLocationCode
	}
	if c.Description == "" && c.Hostname != "" {
		c.Description = fmt.Sprintf("Packer build instance %s", c.Hostname)
	}
	if c.Contract == "" {
		if c.IsSpot {
			c.Contract = "SPOT"
		} else {
			c.Contract = "PAY_AS_YOU_GO"
		}
	}
	if c.PollInterval == 0 {
		c.PollInterval = defaultPollInterval
	}
	if c.InstanceTimeout == 0 {
		c.InstanceTimeout = defaultInstanceTimeout
	}
	if c.APITimeout == 0 {
		c.APITimeout = defaultAPITimeout
	}
	if c.Comm.Type == "" {
		c.Comm.Type = "ssh"
	}
	if c.Comm.Type == "ssh" && c.Comm.SSHUsername == "" {
		c.Comm.SSHUsername = defaultSSHUsername
	}
	if c.TemporarySSHKeyName == "" {
		runID := os.Getenv("PACKER_RUN_UUID")
		if runID == "" {
			runID = uuid.TimeOrderedUUID()
		}
		c.TemporarySSHKeyName = "packer-" + runID
	}
	if len(c.AllowedSSHStatuses) == 0 {
		c.AllowedSSHStatuses = []string{"running"}
	}
	if c.ArtifactType == "" {
		c.ArtifactType = artifactTypeInstance
	}
	if len(c.ArtifactVolumeLocationCodes) > 0 {
		c.ArtifactVolumeLocationCode = c.ArtifactVolumeLocationCodes[0]
	} else {
		if c.ArtifactVolumeLocationCode == "" {
			c.ArtifactVolumeLocationCode = c.LocationCode
		}
		c.ArtifactVolumeLocationCodes = []string{c.ArtifactVolumeLocationCode}
	}
}

func (c *Config) validate() error {
	var errs []error
	required := map[string]string{
		"client_id":        c.ClientID,
		"client_secret":    c.ClientSecret,
		"instance_type":    c.InstanceType,
		"image":            c.Image,
		"hostname":         c.Hostname,
		"description":      c.Description,
		"location_code":    c.LocationCode,
		"contract":         c.Contract,
		"poll_interval":    c.PollInterval.String(),
		"api_timeout":      c.APITimeout.String(),
		"instance_timeout": c.InstanceTimeout.String(),
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" || value == "0s" {
			errs = append(errs, fmt.Errorf("%s must be set", key))
		}
	}
	if c.PollInterval < time.Second {
		errs = append(errs, errors.New("poll_interval must be at least 1s"))
	}
	if c.InstanceTimeout < c.PollInterval {
		errs = append(errs, errors.New("instance_timeout must be greater than poll_interval"))
	}
	if c.APITimeout < time.Second {
		errs = append(errs, errors.New("api_timeout must be at least 1s"))
	}
	if c.StartupScript != "" && c.StartupScriptID != "" {
		errs = append(errs, errors.New("only one of startup_script or startup_script_id can be set"))
	}
	if c.SkipTemporarySSHKey && len(c.SSHKeyIDs) == 0 && c.Comm.Type == "ssh" {
		errs = append(errs, errors.New("ssh_key_ids must be set when skip_temporary_ssh_key is true and communicator is ssh"))
	}
	if c.SkipTemporarySSHKey && c.Comm.Type == "ssh" &&
		c.Comm.SSHPrivateKeyFile == "" && c.Comm.SSHPassword == "" && !c.Comm.SSHAgentAuth {
		errs = append(errs, errors.New("ssh_private_key_file, ssh_password, or ssh_agent_auth must be set when skip_temporary_ssh_key is true"))
	}
	if c.Comm.Type != "ssh" && c.Comm.Type != "none" {
		errs = append(errs, errors.New("only ssh and none communicators are supported"))
	}
	if c.ArtifactType != artifactTypeInstance && c.ArtifactType != artifactTypeOSVolume {
		errs = append(errs, fmt.Errorf("artifact_type must be one of %q or %q", artifactTypeInstance, artifactTypeOSVolume))
	}
	if c.ArtifactType == artifactTypeOSVolume && c.KeepInstance && !c.shouldCloneOSVolume() {
		errs = append(errs, errors.New("keep_instance cannot be true when artifact_type is os_volume and clone_os_volume is false"))
	}
	if c.ArtifactType == artifactTypeOSVolume && !c.shouldCloneOSVolume() && len(c.ArtifactVolumeLocationCodes) > 1 {
		errs = append(errs, errors.New("artifact_volume_location_codes requires clone_os_volume to be true"))
	}
	seenArtifactLocations := make(map[string]struct{}, len(c.ArtifactVolumeLocationCodes))
	for i, location := range c.ArtifactVolumeLocationCodes {
		location = strings.TrimSpace(location)
		if location == "" {
			errs = append(errs, fmt.Errorf("artifact_volume_location_codes.%d must be set", i))
			continue
		}
		if _, ok := seenArtifactLocations[location]; ok {
			errs = append(errs, fmt.Errorf("artifact_volume_location_codes.%d duplicates %q", i, location))
			continue
		}
		seenArtifactLocations[location] = struct{}{}
		c.ArtifactVolumeLocationCodes[i] = location
	}
	for i, volume := range c.Volumes {
		if volume.Name == "" {
			errs = append(errs, fmt.Errorf("volume.%d.name must be set", i))
		}
		if volume.Size <= 0 {
			errs = append(errs, fmt.Errorf("volume.%d.size must be greater than zero", i))
		}
		if volume.Type == "" {
			errs = append(errs, fmt.Errorf("volume.%d.type must be set", i))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func (c *Config) shouldCloneOSVolume() bool {
	if c.CloneOSVolume == nil {
		return true
	}
	return *c.CloneOSVolume
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
