package bundle

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// readIndexPacket reads and validates the index packet at offset 0.
func (b *Bundle) readIndexPacket() error {
	ch, body, err := readAndValidatePacket(b.f, 0, b.size)
	if err != nil {
		return fmt.Errorf("failed to read packet: %w", err)
	}

	if ch.PacketType != PacketTypeIndex {
		return fmt.Errorf("expected index packet at offset 0, got %q", ch.PacketType)
	}

	mp, err := parseIndexPacket(bytes.NewReader(body), ch)
	if err != nil {
		return fmt.Errorf("failed to parse packet: %w", err)
	}

	b.Index = mp

	return nil
}

// readAndValidatePacket reads the common packet header at the given offset and
// validates magic bytes and packet length alignment. Then it reads the rest of
// the packet, validates MD5 and returns packet header, packet body or an error.
func readAndValidatePacket(r io.ReaderAt, offset, fileSize int64) (CommonHeader, []byte, error) {
	// Bounds check: can we fit a header in the remaining file?
	if offset < 0 || fileSize < 0 || offset+commonHeaderSize > fileSize {
		return CommonHeader{}, nil, io.ErrUnexpectedEOF
	}

	// Read the header.
	var hdrBuf [commonHeaderSize]byte
	if _, err := r.ReadAt(hdrBuf[:], offset); err != nil {
		return CommonHeader{}, nil, fmt.Errorf("failed to read header: %w", err)
	}
	var ch CommonHeader
	if err := binary.Read(bytes.NewReader(hdrBuf[:]), binary.LittleEndian, &ch); err != nil {
		return CommonHeader{}, nil, fmt.Errorf("failed to parse header: %w", err)
	}

	// Validate the packet.
	if !isKnownPacketType(ch.PacketType) {
		return CommonHeader{}, nil, fmt.Errorf("%w: %x", ErrUnknownPacketType, ch.PacketType)
	}
	if ch.Magic != Magic {
		return CommonHeader{}, nil, fmt.Errorf("invalid magic bytes: %w", ErrInvalidMagic)
	}
	if ch.PacketLength < commonHeaderSize {
		return CommonHeader{}, nil, fmt.Errorf("invalid packet length %d", ch.PacketLength)
	}
	if !isAligned4(ch.PacketLength) {
		return CommonHeader{}, nil, fmt.Errorf("packet length %d not 4-byte aligned", ch.PacketLength)
	}

	// Bounds check: can we fit the body in the remaining file?
	bodyLen := ch.PacketLength - commonHeaderSize
	bodyOffset := offset + commonHeaderSize
	if uint64(fileSize-bodyOffset) < bodyLen { //nolint:gosec
		return CommonHeader{}, nil, fmt.Errorf("packet length %d exceeds stream size", ch.PacketLength)
	}

	// Read the body at its offset.
	body := make([]byte, bodyLen)
	if _, err := r.ReadAt(body, bodyOffset); err != nil {
		return CommonHeader{}, nil, fmt.Errorf("failed to read body: %w", err)
	}

	// Verify the packet checksum.
	if packetMD5(ch.RecoverySetID, ch.PacketType, body) != ch.PacketMD5 {
		return CommonHeader{}, nil, fmt.Errorf("failed to validate body: %w", ErrInvalidChecksum)
	}

	return ch, body, nil
}
