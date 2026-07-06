packer {
  required_plugins {
    verda = {
      version = ">= 0.0.1"
      source  = "github.com/verda-cloud/verda"
    }
  }
}

source "verda-instance" "ubuntu_volume" {
  instance_type = "V100"
  image         = "ubuntu-24.04"
  hostname      = "packer-verda-volume"

  ssh_username = "root"

  artifact_type        = "os_volume"
  artifact_volume_name = "packer-verda-volume-root"
}

build {
  sources = ["source.verda-instance.ubuntu_volume"]

  provisioner "shell" {
    inline = [
      "cloud-init status --wait || true",
      "apt-get update",
    ]
  }
}
