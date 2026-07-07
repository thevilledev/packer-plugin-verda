packer {
  required_plugins {
    verda = {
      version = ">= 0.1.0"
      source  = "github.com/thevilledev/verda"
    }
  }
}

source "verda-instance" "ubuntu_volume" {
  instance_type = "1A6000.10V"
  location_code = "FIN-01"
  image         = "ubuntu-24.04"
  hostname      = "packer-verda-volume"

  ssh_username = "root"

  artifact_type                  = "os_volume"
  artifact_volume_name           = "packer-verda-volume-root"
  artifact_volume_location_codes = ["FIN-01", "FIN-03"]
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
