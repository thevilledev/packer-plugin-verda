# Packer Plugin Verda

> [!NOTE]
> This is an unofficial implementation. Probably not production ready yet.

`packer-plugin-verda` provides a Packer builder for [Verda Cloud](https://verda.com).

The `verda-instance` builder creates a Verda instance, waits for it to become reachable, runs provisioners, and returns either the created instance or a cloned OS volume artifact.

Uses the official [verdacloud-sdk-go](https://github.com/verda-cloud/verdacloud-sdk-go) and works in tandem with the [terraform-provider-verda](https://github.com/verda-cloud/terraform-provider-verda).

## Installation

```hcl
packer {
  required_plugins {
    verda = {
      version = ">= 0.1.0"
      source  = "github.com/thevilledev/verda"
    }
  }
}
```

Then run:

```sh
packer init .
```

For local development or private repositories, install the built binary directly:

```sh
make plugin-install
```

## Usage

Set credentials with `VERDA_CLIENT_ID` and `VERDA_CLIENT_SECRET`, or pass `client_id` and `client_secret` in the source block.

```hcl
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

To produce a reusable OS volume instead of an instance artifact, set `artifact_type = "os_volume"`. The artifact ID can then be used as the Verda instance `image` value in Terraform or another API client.

Multi-location OS volume builds print every location and volume ID. The OS volume example also writes `VolumeIDsByLocation` to `packer-manifest.json` for machine-readable handoff.

The default instance artifact remains live after a successful build. If Packer later destroys that artifact, the plugin deletes the instance; set `keep_instance = true` to retain it even then. For OS volume artifacts, the temporary build instance is deleted after capture unless `keep_instance` is enabled.


> [!NOTE]
> This step requires extra steps - see [examples/README.md](examples/README.md).

When you deploy from an existing Verda OS volume, do not rely on deployment-time `ssh_key_ids` injection. Bake durable login keys into the guest during the Packer build, as shown in [examples/os-volume.pkr.hcl](examples/os-volume.pkr.hcl), and use `ssh_clear_authorized_keys = true` so Packer's temporary build key is removed before the volume is captured.



## Documentation

- Builder reference: [docs/builders/instance.mdx](docs/builders/instance.mdx)
- Examples: [examples/basic.pkr.hcl](examples/basic.pkr.hcl), [examples/os-volume.pkr.hcl](examples/os-volume.pkr.hcl)

The option reference is generated from the builder config comments with `packer-sdc`, matching the pattern used by other Packer plugins.

## Development

```sh
make tidy-check
make check-fmt
make generate
make test
make lint
make plugin-check
```

`make generate` updates generated HCL2 specs and the HashiCorp integration-ready `.web-docs` output.

Render the docs locally with:

```sh
make renderdocs
```

Build the GitHub Pages site input with:

```sh
make docs-site
```

Run cloud acceptance tests only when you intentionally want to create real Verda resources:

```sh
PACKER_ACC=1 \
VERDA_CLIENT_ID=... \
VERDA_CLIENT_SECRET=... \
VERDA_ACC_INSTANCE_TYPE=... \
VERDA_ACC_IMAGE=... \
make testacc
```

## Release

Releases are built with GoReleaser and use Packer plugin archive naming:

```sh
make snapshot
```

Tagged releases require `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` GitHub secrets so the checksum file can be signed.

## License

MIT.
