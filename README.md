# Packer Plugin Verda

`packer-plugin-verda` provides a Packer builder for Verda Cloud.

The first builder is `verda.instance`. It creates a Verda instance, waits for it to become reachable, and runs standard Packer provisioners over SSH. By default it returns the created instance as the build artifact.

For image-like workflows, set `artifact_type = "os_volume"`. The builder will shut down the provisioned instance, clone its OS volume, and return the cloned volume as the artifact. The current Verda Go SDK does not expose a boot-from-volume field for new instances, so the volume artifact is a preserved block volume, not a native reusable image.

## Requirements

- Go 1.26+
- Packer 1.10+
- Verda client credentials

## Build

```sh
make build
packer plugins install --path ./packer-plugin-verda github.com/verda-cloud/verda
```

## Configuration

Credentials can be provided with `client_id` and `client_secret`, or through `VERDA_CLIENT_ID` and `VERDA_CLIENT_SECRET`.

```hcl
packer {
  required_plugins {
    verda = {
      version = ">= 0.0.1"
      source  = "github.com/verda-cloud/verda"
    }
  }
}

source "verda-instance" "ubuntu" {
  instance_type = "V100"
  image         = "ubuntu-24.04"
  hostname      = "packer-verda-example"

  ssh_username = "root"

  keep_instance      = true
  delete_permanently = false
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

## Important Options

- `instance_type`, `image`, and `hostname` are required.
- `location_code` defaults to `FIN-03`.
- `client_id` and `client_secret` default from `VERDA_CLIENT_ID` and `VERDA_CLIENT_SECRET`.
- `communicator` supports `ssh` and `none`; SSH defaults to user `root`.
- By default the builder creates a temporary Verda SSH key from Packer's generated SSH key and deletes it during cleanup.
- Set `skip_temporary_ssh_key = true` and pass `ssh_key_ids` to use existing Verda SSH keys.
- `artifact_type = "os_volume"` returns a cloned OS volume artifact instead of an instance artifact.
- `clone_os_volume` defaults to `true`; set it to `false` to use the source OS volume directly.
- `artifact_volume_name` controls the cloned OS volume name.
- `skip_shutdown_before_artifact = true` skips the default shutdown before OS volume capture.
- `keep_instance = true` keeps the created instance after the build. The default is to delete it during cleanup.
- `volume_ids_to_delete` controls which volumes are deleted when the instance is deleted.

## Development

```sh
make tidy
make test
make lint
```
