package proxydhcp

import (
	"context"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
	"github.com/rs/zerolog"
)

// FIXME: add AMD64 support
func HandlePkt(ctx context.Context, serverIP net.IP, ipxeTarget string) func(conn net.PacketConn, peer net.Addr, pkt *dhcpv4.DHCPv4) {
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

		if pkt.ClientArch()[0] != iana.EFI_ARM64 {
			zerolog.Ctx(ctx).Info().Msg("packet is not an arm64 request")
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
		userClass := pkt.GetOneOption(dhcpv4.OptionUserClassInformation)
		if string(userClass) == "gobootme" {
			resp.UpdateOption(dhcpv4.OptBootFileName(ipxeTarget))
		} else {
			resp.UpdateOption(dhcpv4.OptTFTPServerName(serverIP.String()))
			resp.UpdateOption(dhcpv4.OptBootFileName("arm64.efi"))
		}

		_, err = conn.WriteTo(resp.ToBytes(), peer)
		if err != nil {
			zerolog.Ctx(ctx).Error().Str("peer", peer.String()).Err(err).Msg("failure sending response")
			return
		}
		zerolog.Ctx(ctx).
			Info().
			Str("source", pkt.ClientHWAddr.String()).
			Str("server", resp.TFTPServerName()).
			Str("boot_filename", resp.BootFileNameOption()).
			Msg("offered boot response")
	}
}
