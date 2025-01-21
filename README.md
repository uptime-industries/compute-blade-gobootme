# gobootme
> While this software heavily focuses on Raspberry Pis (especially for the compute-blade), it also boots AMD64 machines. They are just easier and have less quirks.
> **Disclaimer**: works, but test coverage is lacking!


Bringing Raspberry Pis from zero to iPXE pointing to e.g. [matchbox](http://matchbox.psdn.io) in seconds.

It acts as ProxyDHCP server to provide boot information to the Pis, and then it serves the iPXE boot script to the Pis.

## Requirements/Assumptions
- The host serving the ProxyDHCP server <u>**MUST BE ON THE SAME SUBNET**</u> as the Pi hosts that are making DHCP requests
- DHCP server without PXE booting configured
- Privileged execution

## Getting Started

### Generate Boot Files (MacOS only)
Generate the boot files by running:

```bash
go generate ./...
```

In the root directory of the repository. This creates a minimal set of files in `./internal/tftp/bootfiles`:

```bash
./internal/tftp/bootfiles
├── bcm2711-rpi-4-b.dtb
├── bcm2711-rpi-400.dtb
├── bcm2711-rpi-cm4.dtb
├── config.txt
├── firmware
│   ├── brcm
│   │   ├── brcmfmac43455-sdio.bin
│   │   ├── brcmfmac43455-sdio.clm_blob
│   │   ├── brcmfmac43455-sdio.Raspberry
│   │   └── brcmfmac43455-sdio.txt
│   ├── LICENCE.txt
│   └── Readme.txt
├── fixup4.dat
├── overlays
│   ├── ...
├── RPI_EFI.fd
├── snp.efi
└── start4.elf
```

These boot files are generated inside of a docker image (from the OSS IPXE project), and a <u>custom boot script is embedded</u> into the `snp.efi` binary. This binary bootfile will be served via TFTP.

### Start Services
Run `docker compose up`. This should start both **Matchbox**, and **gobootme** services. 

If you are running MacOS, you unfortunately can't start anything on the DHCP ports without sudo, and will need to do the following:

```bash
> docker compose up matchbox      # start matchbox only
> sudo PROXY_DHCP_INTERFACE=<iface> go run ./cmd/gobootme/main.go
```

### Power on the Pis
Start the Raspberry Pis and watch the console logs from `gobootme` and `matchbox`. It will take a while, but eventually the Pis will be served the PXE files and start trying to communicate with the Matchbox HTTP server.

If you see requests in the matchbox logs, you can [move on to configuring your matchbox installation](https://github.com/poseidon/matchbox/tree/main/examples)!

```
matchbox  | time="2025-01-10T19:43:12Z" level=info msg="Starting matchbox HTTP server on 0.0.0.0:8080"
matchbox  | time="2025-01-10T20:36:21Z" level=info msg="HTTP HEAD /boot.ipxe"
matchbox  | time="2025-01-10T20:36:21Z" level=info msg="HTTP GET /boot.ipxe"
```

**NOTE:** If you make changes to your matchbox assets or data, you will need to restart the docker container for them to take effect!

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

