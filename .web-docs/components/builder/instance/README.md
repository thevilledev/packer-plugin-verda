Type: `verda-instance`

Artifact BuilderId: `verda.instance`

The `verda-instance` builder creates a Verda instance, waits for it to become reachable, and runs standard Packer provisioners over SSH.

By default the artifact is the created instance. With `artifact_type = "os_volume"`, the builder shuts down the instance, clones its OS volume, and returns the cloned volume artifact. The returned OS volume ID can be used later as the Verda instance `image` value in Terraform or another API client.

## Configuration Reference

### Required

<!-- Code generated from the comments of the Config struct in builder/instance/config.go; DO NOT EDIT MANUALLY -->

- `client_id` (string) - Verda client ID. It can also be set with VERDA_CLIENT_ID.

- `client_secret` (string) - Verda client secret. It can also be set with VERDA_CLIENT_SECRET.

- `instance_type` (string) - Verda instance type to create.

- `image` (string) - Image name, image ID, or OS volume ID to boot from.

- `hostname` (string) - Hostname for the created instance.

<!-- End of code generated from the comments of the Config struct in builder/instance/config.go; -->


### Optional

<!-- Code generated from the comments of the Config struct in builder/instance/config.go; DO NOT EDIT MANUALLY -->

- `base_url` (string) - Verda API base URL. Leave unset for the production API.

- `debug` (bool) - Enable verbose Verda SDK logging.

- `description` (string) - Description for the created instance. Defaults to a Packer build description based on the hostname.

- `location_code` (string) - Verda location code for the instance. Defaults to FIN-03.

- `contract` (string) - Instance contract. Defaults to PAY_AS_YOU_GO, or SPOT when is_spot is true.

- `pricing` (string) - Optional pricing value passed to the Verda API.

- `is_spot` (bool) - Request a spot instance. When true, the default contract is SPOT.

- `coupon` (string) - Optional coupon value passed to the Verda API.

- `ssh_key_ids` ([]string) - Existing Verda SSH key IDs to add to the instance.

- `temporary_ssh_key_name` (string) - Name for the temporary Verda SSH key created from Packer's generated public key.

- `skip_temporary_ssh_key` (bool) - Disable temporary Verda SSH key creation. When true with SSH, provide ssh_key_ids and a matching SSH credential.

- `startup_script_id` (string) - Existing Verda startup script ID to attach to the instance.

- `startup_script` (string) - Startup script content to create before launching the instance.

- `startup_script_name` (string) - Name for a startup script created from startup_script.

- `delete_startup_script` (bool) - Delete a startup script created from startup_script during cleanup.

- `existing_volume_ids` ([]string) - Existing non-OS volume IDs to attach to the instance.

- `os_volume_name` (string) - Name for the instance OS volume.

- `os_volume_size` (int) - Size, in GiB, for the instance OS volume.

- `os_volume_spot_behavior` (string) - Spot discontinuation behavior for the instance OS volume.

- `volume` ([]Volume) - Additional data volumes to create with the instance.

- `artifact_type` (string) - Artifact to return from the build. Valid values are instance and os_volume. Defaults to instance.

- `clone_os_volume` (\*bool) - Clone the source OS volume when artifact_type is os_volume. Defaults to true.

- `artifact_volume_name` (string) - Name for each cloned OS volume artifact.

- `artifact_volume_location_code` (string) - Target location for a cloned OS volume artifact. Defaults to location_code, and the source location is also retained.

- `artifact_volume_location_codes` ([]string) - Target locations for cloned OS volume artifacts. The first location becomes the primary artifact ID, and the source location is also retained.

- `skip_shutdown_before_artifact` (bool) - Skip shutting down the instance before creating an OS volume artifact.

- `keep_instance` (bool) - Keep the created instance after the build. By default the instance is deleted during cleanup.

- `delete_permanently` (bool) - Permanently delete resources during cleanup instead of moving them to a recoverable state.

- `volume_ids_to_delete` ([]string) - Volume IDs to delete when the instance is deleted.

- `poll_interval` (duration string | ex: "1h5m2s") - Interval between Verda instance status checks. Defaults to 15s.

- `instance_timeout` (duration string | ex: "1h5m2s") - Timeout for the instance to become reachable. Defaults to 30m.

- `api_timeout` (duration string | ex: "1h5m2s") - HTTP client timeout for Verda API calls. Defaults to 10m.

- `allowed_ssh_statuses` ([]string) - Instance statuses that are acceptable for SSH connection attempts. Defaults to running.

<!-- End of code generated from the comments of the Config struct in builder/instance/config.go; -->


### Additional Volume Blocks

Use `volume` blocks to create additional non-OS data volumes with the instance.

Required `volume` fields:

<!-- Code generated from the comments of the Volume struct in builder/instance/config.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Name for the additional volume.

- `size` (int) - Size, in GiB, for the additional volume.

- `type` (string) - Volume type.

<!-- End of code generated from the comments of the Volume struct in builder/instance/config.go; -->


Optional `volume` fields:

<!-- Code generated from the comments of the Volume struct in builder/instance/config.go; DO NOT EDIT MANUALLY -->

- `location_code` (string) - Verda location code for the additional volume. Defaults to the instance location when unset.

- `on_spot_discontinue` (string) - Spot discontinuation behavior for the additional volume.

<!-- End of code generated from the comments of the Volume struct in builder/instance/config.go; -->


### Communicator Config

This builder supports the `ssh` and `none` communicators. SSH is the default and uses `root` as the default username.

Most SSH options below are inherited from Packer's standard SSH communicator. The Verda-specific SSH settings are `ssh_key_ids`, `temporary_ssh_key_name`, and `skip_temporary_ssh_key`. By default the builder generates a temporary Packer key pair, creates a temporary Verda SSH key from its public key, passes that key ID when creating the build instance, and deletes the Verda key during cleanup.

<!-- Code generated from the comments of the Config struct in communicator/config.go; DO NOT EDIT MANUALLY -->

- `communicator` (string) - Packer currently supports three kinds of communicators:
  
  -   `none` - No communicator will be used. If this is set, most
      provisioners also can't be used.
  
  -   `ssh` - An SSH connection will be established to the machine. This
      is usually the default.
  
  -   `winrm` - A WinRM connection will be established.
  
  In addition to the above, some builders have custom communicators they
  can use. For example, the Docker builder has a "docker" communicator
  that uses `docker exec` and `docker cp` to execute scripts and copy
  files.

- `pause_before_connecting` (duration string | ex: "1h5m2s") - We recommend that you enable SSH or WinRM as the very last step in your
  guest's bootstrap script, but sometimes you may have a race condition
  where you need Packer to wait before attempting to connect to your
  guest.
  
  If you end up in this situation, you can use the template option
  `pause_before_connecting`. By default, there is no pause. For example if
  you set `pause_before_connecting` to `10m` Packer will check whether it
  can connect, as normal. But once a connection attempt is successful, it
  will disconnect and then wait 10 minutes before connecting to the guest
  and beginning provisioning.

<!-- End of code generated from the comments of the Config struct in communicator/config.go; -->


<!-- Code generated from the comments of the SSHTemporaryKeyPair struct in communicator/config.go; DO NOT EDIT MANUALLY -->

- `temporary_key_pair_type` (string) - `dsa` | `ecdsa` | `ed25519` | `rsa` ( the default )
  
  Specifies the type of key to create. The possible values are 'dsa',
  'ecdsa', 'ed25519', or 'rsa'.
  
  NOTE: DSA is deprecated and no longer recognized as secure, please
  consider other alternatives like RSA or ED25519.

- `temporary_key_pair_bits` (int) - Specifies the number of bits in the key to create. For RSA keys, the
  minimum size is 1024 bits and the default is 4096 bits. Generally, 3072
  bits is considered sufficient. DSA keys must be exactly 1024 bits as
  specified by FIPS 186-2. For ECDSA keys, bits determines the key length
  by selecting from one of three elliptic curve sizes: 256, 384 or 521
  bits. Attempting to use bit lengths other than these three values for
  ECDSA keys will fail. Ed25519 keys have a fixed length and bits will be
  ignored.
  
  NOTE: DSA is deprecated and no longer recognized as secure as specified
  by FIPS 186-5, please consider other alternatives like RSA or ED25519.

<!-- End of code generated from the comments of the SSHTemporaryKeyPair struct in communicator/config.go; -->


<!-- Code generated from the comments of the SSH struct in communicator/config.go; DO NOT EDIT MANUALLY -->

- `ssh_host` (string) - The address to SSH to. This usually is automatically configured by the
  builder.

- `ssh_port` (int) - The port to connect to SSH. This defaults to `22`.

- `ssh_username` (string) - The username to connect to SSH with. Required if using SSH.

- `ssh_password` (string) - A plaintext password to use to authenticate with SSH.

- `ssh_ciphers` ([]string) - This overrides the value of ciphers supported by default by Golang.
  The default value is [
    "aes128-gcm@openssh.com",
    "chacha20-poly1305@openssh.com",
    "aes128-ctr", "aes192-ctr", "aes256-ctr",
  ]
  
  Valid options for ciphers include:
  "aes128-ctr", "aes192-ctr", "aes256-ctr", "aes128-gcm@openssh.com",
  "chacha20-poly1305@openssh.com",
  "arcfour256", "arcfour128", "arcfour", "aes128-cbc", "3des-cbc",

- `ssh_clear_authorized_keys` (bool) - If true, Packer will attempt to remove its temporary key from
  `~/.ssh/authorized_keys` and `/root/.ssh/authorized_keys`. This is a
  mostly cosmetic option, since Packer will delete the temporary private
  key from the host system regardless of whether this is set to true
  (unless the user has set the `-debug` flag). Defaults to "false";
  currently only works on guests with `sed` installed.

- `ssh_key_exchange_algorithms` ([]string) - If set, Packer will override the value of key exchange (kex) algorithms
  supported by default by Golang. Acceptable values include:
  "curve25519-sha256@libssh.org", "ecdh-sha2-nistp256",
  "ecdh-sha2-nistp384", "ecdh-sha2-nistp521",
  "diffie-hellman-group14-sha1", and "diffie-hellman-group1-sha1".

- `ssh_certificate_file` (string) - Path to user certificate used to authenticate with SSH.
  The `~` can be used in path and will be expanded to the
  home directory of current user.

- `ssh_pty` (bool) - If `true`, a PTY will be requested for the SSH connection. This defaults
  to `false`.

- `ssh_timeout` (duration string | ex: "1h5m2s") - The time to wait for SSH to become available. Packer uses this to
  determine when the machine has booted so this is usually quite long.
  Example value: `10m`.
  This defaults to `5m`, unless `ssh_handshake_attempts` is set.

- `ssh_disable_agent_forwarding` (bool) - If true, SSH agent forwarding will be disabled. Defaults to `false`.

- `ssh_handshake_attempts` (int) - The number of handshakes to attempt with SSH once it can connect.
  This defaults to `10`, unless a `ssh_timeout` is set.

- `ssh_bastion_host` (string) - A bastion host to use for the actual SSH connection.

- `ssh_bastion_port` (int) - The port of the bastion host. Defaults to `22`.

- `ssh_bastion_agent_auth` (bool) - If `true`, the local SSH agent will be used to authenticate with the
  bastion host. Defaults to `false`.

- `ssh_bastion_username` (string) - The username to connect to the bastion host.

- `ssh_bastion_password` (string) - The password to use to authenticate with the bastion host.

- `ssh_bastion_interactive` (bool) - If `true`, the keyboard-interactive used to authenticate with bastion host.

- `ssh_bastion_private_key_file` (string) - Path to a PEM encoded private key file to use to authenticate with the
  bastion host. The `~` can be used in path and will be expanded to the
  home directory of current user.

- `ssh_bastion_certificate_file` (string) - Path to user certificate used to authenticate with bastion host.
  The `~` can be used in path and will be expanded to the
  home directory of current user.

- `ssh_file_transfer_method` (string) - `scp` or `sftp` - How to transfer files, Secure copy (default) or SSH
  File Transfer Protocol.
  
  **NOTE**: Guests using Windows with Win32-OpenSSH v9.1.0.0p1-Beta, scp
  (the default protocol for copying data) returns a a non-zero error code since the MOTW
  cannot be set, which cause any file transfer to fail. As a workaround you can override the transfer protocol
  with SFTP instead `ssh_file_transfer_method = "sftp"`.

- `ssh_proxy_host` (string) - A SOCKS proxy host to use for SSH connection

- `ssh_proxy_port` (int) - A port of the SOCKS proxy. Defaults to `1080`.

- `ssh_proxy_username` (string) - The optional username to authenticate with the proxy server.

- `ssh_proxy_password` (string) - The optional password to use to authenticate with the proxy server.

- `ssh_keep_alive_interval` (duration string | ex: "1h5m2s") - How often to send "keep alive" messages to the server. Set to a negative
  value (`-1s`) to disable. Example value: `10s`. Defaults to `5s`.

- `ssh_read_write_timeout` (duration string | ex: "1h5m2s") - The amount of time to wait for a remote command to end. This might be
  useful if, for example, packer hangs on a connection after a reboot.
  Example: `5m`. Disabled by default.

- `ssh_remote_tunnels` ([]string) - Remote tunnels forward a port from your local machine to the instance.
  Format: ["REMOTE_PORT:LOCAL_HOST:LOCAL_PORT"]
  Example: "9090:localhost:80" forwards localhost:9090 on your machine to port 80 on the instance.

- `ssh_local_tunnels` ([]string) - Local tunnels forward a port from the instance to your local machine.
  Format: ["LOCAL_PORT:REMOTE_HOST:REMOTE_PORT"]
  Example: "8080:localhost:3000" allows the instance to access your local machine’s port 3000 via localhost:8080.

<!-- End of code generated from the comments of the SSH struct in communicator/config.go; -->


- `ssh_private_key_file` (string) - Path to a PEM encoded private key file to use to authenticate with SSH.
  The `~` can be used in path and will be expanded to the home directory
  of current user.


## Basic Example

```hcl
packer {
  required_plugins {
    verda = {
      version = ">= 0.1.0"
      source  = "github.com/thevilledev/verda"
    }
  }
}

source "verda-instance" "ubuntu" {
  instance_type = "1L40S.20V"
  location_code = "FIN-03"
  image         = "ubuntu-24.04"
  hostname      = "packer-verda-example"

  ssh_username = "root"
}

build {
  sources = ["source.verda-instance.ubuntu"]

  provisioner "shell" {
    inline = [
      "cloud-init status --wait || true",
      "uname -a",
    ]
  }
}
```

## OS Volume Artifacts

Set `artifact_type = "os_volume"` to return a cloned OS volume instead of the instance. For multi-location fleets, set `artifact_volume_location_codes` to clone the artifact volume to each target location; the source location is also retained as a cloned artifact. The first configured location is the primary artifact ID, and generated data exposes all volume IDs by location.

Instances booted later from an existing OS volume cannot rely on Verda `ssh_key_ids` injection. Bake durable login keys into the guest during the Packer build, and enable `ssh_clear_authorized_keys` so Packer's temporary build key is removed before the volume is captured.

```hcl
variable "authorized_keys_file" {
  type        = string
  description = "Path to an authorized_keys-format file to bake into the OS volume."
}

source "verda-instance" "ubuntu_volume" {
  instance_type = "1A6000.10V"
  location_code = "FIN-01"
  image         = "ubuntu-24.04"
  hostname      = "packer-verda-volume"

  ssh_username              = "root"
  ssh_clear_authorized_keys = true

  artifact_type                  = "os_volume"
  artifact_volume_name           = "packer-verda-volume-root"
  artifact_volume_location_codes = ["FIN-01", "FIN-03"]
}

build {
  sources = ["source.verda-instance.ubuntu_volume"]

  provisioner "file" {
    source      = var.authorized_keys_file
    destination = "/tmp/verda_authorized_keys"
  }

  provisioner "shell" {
    inline = [
      "cloud-init status --wait || true",
      "install -d -m 0700 /root/.ssh",
      "touch /root/.ssh/authorized_keys",
      "chmod 0600 /root/.ssh/authorized_keys",
      "while IFS= read -r key; do [ -n \"$key\" ] || continue; grep -qxF \"$key\" /root/.ssh/authorized_keys || printf '%s\\n' \"$key\" >> /root/.ssh/authorized_keys; done < /tmp/verda_authorized_keys",
      "chown root:root /root/.ssh /root/.ssh/authorized_keys",
      "rm -f /tmp/verda_authorized_keys",
    ]
  }
}
```

## Terraform Handoff

Use the Packer OS volume artifact ID, or a clone of it, as the Terraform `verda_instance.image` value. Do not pass it through `existing_volumes`; that field attaches additional non-OS volumes. Do not rely on `ssh_key_ids` here when booting from an existing OS volume; the keys must already be present in the guest image, or installed by a boot script that runs inside the guest.

For multiple locations, store the generated `VolumeIDsByLocation` map and select the volume ID matching each instance location. For multiple instances in the same location, clone that location's Packer artifact once per instance before passing each clone ID as `image`.

```hcl
variable "os_volume_ids_by_location" {
  description = "Packer VolumeIDsByLocation generated data."
  type        = map(string)
}

resource "verda_instance" "app" {
  instance_type = "1B200.30V"
  image         = var.os_volume_ids_by_location["FIN-03"]
  hostname      = "app-01"
  description   = "App server from Packer OS volume"
  location      = "FIN-03"
  ssh_key_ids   = []
}
```

Verda volumes do not expose tags, labels, or arbitrary metadata through the public API, Terraform provider, or Go SDK. Put release identifiers in the volume name or track them in external inventory.
