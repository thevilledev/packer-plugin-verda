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

	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	BaseURL      string `mapstructure:"base_url"`
	Debug        bool   `mapstructure:"debug"`

	InstanceType string `mapstructure:"instance_type"`
	Image        string `mapstructure:"image"`
	Hostname     string `mapstructure:"hostname"`
	Description  string `mapstructure:"description"`
	LocationCode string `mapstructure:"location_code"`
	Contract     string `mapstructure:"contract"`
	Pricing      string `mapstructure:"pricing"`
	IsSpot       bool   `mapstructure:"is_spot"`
	Coupon       string `mapstructure:"coupon"`

	SSHKeyIDs            []string `mapstructure:"ssh_key_ids"`
	TemporarySSHKeyName  string   `mapstructure:"temporary_ssh_key_name"`
	SkipTemporarySSHKey  bool     `mapstructure:"skip_temporary_ssh_key"`
	StartupScriptID      string   `mapstructure:"startup_script_id"`
	StartupScript        string   `mapstructure:"startup_script"`
	StartupScriptName    string   `mapstructure:"startup_script_name"`
	DeleteStartupScript  bool     `mapstructure:"delete_startup_script"`
	ExistingVolumeIDs    []string `mapstructure:"existing_volume_ids"`
	OSVolumeName         string   `mapstructure:"os_volume_name"`
	OSVolumeSize         int      `mapstructure:"os_volume_size"`
	OSVolumeSpotBehavior string   `mapstructure:"os_volume_spot_behavior"`
	Volumes              []Volume `mapstructure:"volume"`

	ArtifactType               string `mapstructure:"artifact_type"`
	CloneOSVolume              *bool  `mapstructure:"clone_os_volume"`
	ArtifactVolumeName         string `mapstructure:"artifact_volume_name"`
	ArtifactVolumeLocationCode string `mapstructure:"artifact_volume_location_code"`
	SkipShutdownBeforeArtifact bool   `mapstructure:"skip_shutdown_before_artifact"`

	KeepInstance       bool          `mapstructure:"keep_instance"`
	DeletePermanently  bool          `mapstructure:"delete_permanently"`
	VolumeIDsToDelete  []string      `mapstructure:"volume_ids_to_delete"`
	PollInterval       time.Duration `mapstructure:"poll_interval"`
	InstanceTimeout    time.Duration `mapstructure:"instance_timeout"`
	APITimeout         time.Duration `mapstructure:"api_timeout"`
	AllowedSSHStatuses []string      `mapstructure:"allowed_ssh_statuses"`
}

// Volume describes an additional data volume to create with the instance.
type Volume struct {
	Name              string `mapstructure:"name"`
	Size              int    `mapstructure:"size"`
	Type              string `mapstructure:"type"`
	LocationCode      string `mapstructure:"location_code"`
	OnSpotDiscontinue string `mapstructure:"on_spot_discontinue"`
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
		"VolumeLocation",
		"VolumeName",
		"VolumeStatus",
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
	if c.ArtifactVolumeLocationCode == "" {
		c.ArtifactVolumeLocationCode = c.LocationCode
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
