package tftp

import (
	"embed"
	"io"
	"path"

	"github.com/google/uuid"
	"github.com/pin/tftp/v3"
	"github.com/rs/zerolog/log"
)

//go:generate docker buildx build --output bootfiles .

//go:embed bootfiles
var bootFiles embed.FS

// noopWriteHandler
func noopWriteHandler(_ string, _ io.WriterTo) error {
	return nil
}

// readHandler
func readHandler(filename string, rf io.ReaderFrom) error {
	logger := log.With().Str("transactionId", uuid.NewString()).Logger()

	actualFilename := filename
	if path.Base(filename) != filename {
		actualFilename = path.Base(filename)
	}

	logger.Info().
		Str("requested", filename).
		Str("resolved", actualFilename).
		Msg("got tftp read request")

	f, err := bootFiles.Open(path.Join("bootfiles", actualFilename))
	if err != nil {
		logger.Error().Err(err).Str("filename", filename).Msg("failed to open file")
		return err
	}
	defer f.Close()
	numBytes, err := rf.ReadFrom(f)
	if err != nil {
		logger.Error().Err(err).Str("filename", filename).Msg("failed to read file")
		return err
	}
	logger.Info().Int64("bytes", numBytes).Str("filename", filename).Msg("served file")

	return nil
}

// New tftp server that serves the iPXE files
func NewIpxeServer() *tftp.Server {
	return tftp.NewServer(readHandler, noopWriteHandler)
}
