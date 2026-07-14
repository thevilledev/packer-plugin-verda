packer {
  required_plugins {
    verda = {
      version = ">= 0.1.0"
      source  = "github.com/thevilledev/verda"
    }
  }
}

source "verda-instance" "ubuntu_spot" {
  instance_type = "1L40S.20V"
  location_code = "FIN-03"
  image         = "ubuntu-24.04"
  hostname      = "packer-verda-spot-example"

  is_spot = true

  ssh_username = "root"
}

build {
  sources = ["source.verda-instance.ubuntu_spot"]

  provisioner "shell" {
    inline = [
      "cloud-init status --wait || true",
      "uname -a",
    ]
  }
}
