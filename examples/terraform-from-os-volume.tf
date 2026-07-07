terraform {
  required_providers {
    verda = {
      source  = "verda-cloud/verda"
      version = ">= 1.1.2"
    }
  }
}

provider "verda" {}

variable "os_volume_id" {
  description = "Packer OS volume artifact ID, or a per-instance clone of it. The volume should already contain login SSH keys."
  type        = string
}

resource "verda_instance" "app" {
  instance_type = "1B200.30V"
  image         = var.os_volume_id
  hostname      = "app-from-packer-volume"
  description   = "Instance created from a Packer-built Verda OS volume"
  location      = "FIN-03"

  # Verda does not inject ssh_key_ids when booting from an existing OS volume.
  # Bake /root/.ssh/authorized_keys into the volume during the Packer build.
}
