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
  description = "Packer OS volume artifact ID, or a per-instance clone of it."
  type        = string
}

resource "verda_ssh_key" "main" {
  name       = "packer-volume-example"
  public_key = file("~/.ssh/id_ed25519.pub")
}

resource "verda_instance" "app" {
  instance_type = "1B200.30V"
  image         = var.os_volume_id
  hostname      = "app-from-packer-volume"
  description   = "Instance created from a Packer-built Verda OS volume"
  location      = "FIN-03"

  ssh_key_ids = [verda_ssh_key.main.id]
}
