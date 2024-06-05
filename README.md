# gobootme
> While this software heavily focuses on Raspberry Pis (especially for the compute-blade), it also boots AMD64 machines. They are just easier and have less quirks.
> **Disclaimer**: works, but test coverage is lacking!


Bringing Raspberry Pis from zero to iPXE pointing to e.g. [matchbox](http://matchbox.psdn.io) in seconds.

It acts as ProxyDHCP server to provide boot information to the Pis, and then it serves the iPXE boot script to the Pis.

## Requirements/Assumptions
- DHCP server without PXE booting configured
- Privileged execution

## Recommendations
While this software tries to boot the Pis from zero, it's recommended to use the UEFI firmware for the most reliable booting experience.

## Configuration
- `LOG_LEVEL`: Log level (**default: `info`**)
- `LOG_MODE`: Log mode (**default: `console`**)
- `IPXE_BOOT_ENDPOINT_AUTO`: Expect matchbox running on the same host (**default: `true`**)
- `IPXE_BOOT_ENDPOINT`: custom iPXE boot script endpoint (only used if `IPXE_BOOT_ENDPOINT_AUTO` is `false`)
- `ENABLE_PROXY_DHCP`: Enable proxy DHCP server (**default: `true`**)
- `PROXY_DHCP_INTERFACE`: Interface to listen for bootp requests (**default: `eth0`**)

## Example runtime

e.g. bringing up matchbox and gobootme in docker-compose
```yaml
version: '3.8'

services:
  gobootme:
    image: ghcr.io/xvzf/gobootme:latest
    container_name: gobootme
    restart: unless-stopped
    network_mode: host
    environment:
      - LOG_LEVEL=info
      - LOG_MODE=json
      - IPXE_BOOT_ENDPOINT_AUTO=true
      - ENABLE_PROXY_DHCP=true
      - PROXY_DHCP_INTERFACE=eth0

  matchbox:
    image: quay.io/poseidon/matchbox:latest
    container_name: matchbox
    restart: unless-stopped
    args:
      - -address=0.0.0.0:8080
      - -log-level=debug
    volumes:
      - ./matchbox/assets:/var/lib/matchbox/assets
      - ./matchbox/data:/var/lib/matchbox/data
      - ./matchbox/rules:/var/lib/matchbox/rules
    ports:
      - "8080:8080"  # Example port mapping, adjust as needed
```

