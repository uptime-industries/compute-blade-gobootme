#cloud-config
hostname: orchestrator-bootstrapper
manage_etc_hosts: true

users:
  - name: admin
    gecos: "Admin User"
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - "ssh-rsa AAAAB3Nza..."

package_update: true
package_upgrade: true

packages:
  - curl
  - vim
  - htop

runcmd:
  - echo "Downloading and installing orchestrator-backend and orchestrator-agent packages..."
  - wget -q -O /tmp/orchestrator-backend.deb http://192.168.11.151:8080/assets/orchestrator-backend.deb # TODO: Replace with GH asset package URL
  - wget -q -O /tmp/orchestrator-agent.deb http://192.168.11.151:8080/assets/orchestrator-agent.deb # TODO: Replace with GH asset package URL
  - dpkg -i /tmp/orchestrator-backend.deb
  - dpkg -i /tmp/orchestrator-agent.deb
  - systemctl enable orchestrator-backend
  - systemctl enable orchestrator-agent
  - systemctl start orchestrator-backend
  - systemctl start orchestrator-agent
