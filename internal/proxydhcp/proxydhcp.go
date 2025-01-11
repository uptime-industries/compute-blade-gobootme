package proxydhcp

import (
	"context"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
	"github.com/rs/zerolog"
)

func isRaspberryPiEEPROM(pkt *dhcpv4.DHCPv4) bool {
	// Legacy boot option is used by the EEPROM
	if pkt.ClientArch()[0] != iana.INTEL_X86PC {
		return false
	}

	// Check if mac address is from a Raspberry Pi
	macPrefix := pkt.ClientHWAddr.String()[0:8]

	// Warning: This might be incomplete but we don't really have a better way
	// Taken from https://maclookup.app/vendors/raspberry-pi-trading-ltd (01.06.2024)
	switch macPrefix {
	case "28:cd:c1":
		return true
	case "2c:cf:67":
		return true
	case "3a:35:41":
		return true
	case "d8:3a:dd":
		return true
	case "dc:a6:32":
		return true
	case "e4:5f:01":
		return true
	}

	return false
}

func HandlePkt(
	ctx context.Context,
	serverIP net.IP,
	ipxeTarget string,
	efiBootfiles string,
) func(conn net.PacketConn, peer net.Addr, pkt *dhcpv4.DHCPv4) {
	return func(conn net.PacketConn, peer net.Addr, pkt *dhcpv4.DHCPv4) {
		if pkt.MessageType() != dhcpv4.MessageTypeDiscover {
			zerolog.Ctx(ctx).
				Info().
				Str("got", pkt.MessageType().String()).
				Str("expected", dhcpv4.MessageTypeDiscover.String()).
				Msg("unexpected message type")
			return
		}

		if pkt.Options[93] == nil {
			zerolog.Ctx(ctx).Info().Msg("packet is not a PXE boot request")
			return
		}

		// Check if guid is ok
		guid := pkt.GetOneOption(dhcpv4.OptionClientMachineIdentifier)
		if len(guid) != 17 {
			zerolog.Ctx(ctx).Info().Msg("packet has invalid GUID length")
			return
		}
		if guid[0] != 0x00 {
			zerolog.Ctx(ctx).Info().Msg("malformed GUID, first byte is not 0x00")
			return
		}

		arch := pkt.ClientArch()[0]
		if !isRaspberryPiEEPROM(pkt) && !(arch == iana.EFI_ARM64 || arch == iana.EFI_ARM64_HTTP) {
			zerolog.Ctx(ctx).Info().
				Str("arch", pkt.ClientArch()[0].String()).
				Str("client ip", pkt.ClientIPAddr.String()).
				Str("client mac", pkt.ClientHWAddr.String()).
				Msg("packet is not an arm64 UEFI / HTTP request or coming from the RaspberryPi EEPROM")
			return
		}

		replyModifiers := []dhcpv4.Modifier{
			dhcpv4.WithServerIP(serverIP),
			dhcpv4.WithOptionCopied(pkt, dhcpv4.OptionClientMachineIdentifier),
			dhcpv4.WithOptionCopied(pkt, dhcpv4.OptionClassIdentifier),
		}

		resp, err := dhcpv4.NewReplyFromRequest(pkt, replyModifiers...)
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Msg("failed to construct DHCP offer")
			return
		}

		if resp.GetOneOption(dhcpv4.OptionClassIdentifier) == nil {
			resp.UpdateOption(dhcpv4.OptClassIdentifier("PXEClient"))
		}

		// Check if the request is coming from our custom iPXE script
		// Override the boot filename with the http target script
		// Override the HTTP boot filename since we can target matchbox directly too
		userClass := pkt.GetOneOption(dhcpv4.OptionUserClassInformation)
		if string(userClass) == "gobootme" {
			resp.UpdateOption(dhcpv4.OptBootFileName(ipxeTarget))
		} else if arch == iana.EFI_ARM64_HTTP {
			resp.UpdateOption(dhcpv4.OptBootFileName(efiBootfiles))
		} else {
			// Set the TFTP server IP for any other request
			resp.UpdateOption(dhcpv4.OptTFTPServerName(serverIP.String()))
			resp.UpdateOption(dhcpv4.OptBootFileName("snp.efi"))
		}

		_, err = conn.WriteTo(resp.ToBytes(), peer)
		if err != nil {
			zerolog.Ctx(ctx).Error().Str("peer", peer.String()).Err(err).Msg("failure sending response")
			return
		}
		zerolog.Ctx(ctx).
			Info().
			Str("arch", pkt.ClientArch()[0].String()).
			Str("source", pkt.ClientHWAddr.String()).
			Str("server", resp.TFTPServerName()).
			Str("boot_filename", resp.BootFileNameOption()).
			Bool("is_raspberry_pi_eeprom", isRaspberryPiEEPROM(pkt)).
			Msg("offered boot response")
	}
}
