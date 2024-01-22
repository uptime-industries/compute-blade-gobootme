package tftp

import (
	"embed"
	"io"
	"path"

	"github.com/pin/tftp/v3"
)

//go:generate docker buildx build --output ipxe .

//go:embed ipxe
var ipxeFiles embed.FS

// noopWriteHandler
func noopWriteHandler(_ string, _ io.WriterTo) error {
	return nil
}

// readHandler
func readHandler(filename string, rf io.ReaderFrom) error {
	f, err := ipxeFiles.Open(path.Join("ipxe", filename))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = rf.ReadFrom(f)
	return err
}

// New tftp server that serves the iPXE files
func NewIpxeServer() *tftp.Server {
	return tftp.NewServer(readHandler, noopWriteHandler)
}
