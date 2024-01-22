# gobootme
> API driven (i)PXE booting for Raspberry Pis and more. Inspired by [dananderson/netboot](https://github.com/danderson/netboot).

## Configuration
- `IPXE_BOOT_ENDPOINT`: HTTP endpoint called when a device tries to (i)PXE boot
- `PROXY_DHCP_INTERFACE`: Interface to listen for bootp requests (**default: `eth0`**)
- `HTTP_PORT`: internal http port for serving iPXE scripts (**default: `8082`**)

## Running the service
Run the provided container image or binary on a linux host
