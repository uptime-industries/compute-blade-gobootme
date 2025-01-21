#cloud-config

# Create orchestrator user
users:
  - name: orchestrator
    shell: /bin/bash
    sudo: ALL=(ALL) NOPASSWD:ALL
    groups: wheel
    home: /home/orchestrator

# Install necessary utilities
packages:
  - curl
  - wget
  - tar
  - gzip
  - dpkg
  - systemd
  - raspberrypi-firmware  # Install raspberrypi-firmware for vcgencmd

# Run commands to get Raspberry Pi information and setup
runcmd:
  - echo "Gathering Raspberry Pi information..."
  - uname -a > /home/orchestrator/pi_info.txt
  - vcgencmd measure_temp > /home/orchestrator/pi_temp.txt
  - vcgencmd get_mem arm > /home/orchestrator/pi_memory.txt
  - vcgencmd get_mem gpu > /home/orchestrator/pi_gpu_memory.txt
  - echo "Raspberry Pi info collected."

# Download the .deb files from GitHub
  - echo "Downloading Debian packages from GitHub..."
  - curl -L https://github.com/yourusername/yourrepository/releases/download/v1.0/orchestrator-backend.deb -o /home/orchestrator/orchestrator-backend.deb
  - curl -L https://github.com/yourusername/yourrepository/releases/download/v1.0/orchestrator-agent.deb -o /home/orchestrator/orchestrator-agent.deb
  - echo "Debian packages downloaded."

# Install the downloaded .deb packages
  - echo "Installing orchestrator-backend.deb..."
  - dpkg -i /home/orchestrator/orchestrator-backend.deb
  - echo "Installing orchestrator-agent.deb..."
  - dpkg -i /home/orchestrator/orchestrator-agent.deb

# Fix any missing dependencies if necessary
  - echo "Fixing any missing dependencies..."
  - apt-get install -f -y

# Enable and start the services
  - echo "Enabling and starting orchestrator-backend service..."
  - systemctl enable orchestrator-backend
  - systemctl start orchestrator-backend
  - echo "Enabling and starting orchestrator-agent service..."
  - systemctl enable orchestrator-agent
  - systemctl start orchestrator-agent

# Set file ownership to orchestrator user
  - chown -R orchestrator:orchestrator /home/orchestrator
