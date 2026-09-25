package thumbs

import (
	"bytes"
	"encoding/binary"
)

var exifPrefix = []byte("Exif\x00\x00")

// jpegOrientation находит тег Orientation (0x0112) в сегменте APP1 JPEG.
// При любой неоднозначности возвращает 1 (нормальная ориентация).
func jpegOrientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(b) {
		if b[i] != 0xFF {
			return 1
		}
		marker := b[i+1]
		switch {
		case marker == 0xFF:
			i++
			continue
		case marker == 0x01 || (marker >= 0xD0 && marker <= 0xD8):
			i += 2
			continue
		case marker == 0xD9 || marker == 0xDA:
			return 1
		}
		segLen := int(binary.BigEndian.Uint16(b[i+2:]))
		if segLen < 2 {
			return 1
		}
		if marker == 0xE1 {
			seg := b[i+4:]
			if end := i + 2 + segLen; end <= len(b) {
				seg = b[i+4 : end]
			}
			if bytes.HasPrefix(seg, exifPrefix) {
				return tiffOrientation(seg[len(exifPrefix):])
			}
		}
		i += 2 + segLen
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	if bo.Uint16(t[2:]) != 42 {
		return 1
	}
	off := int(bo.Uint32(t[4:]))
	if off < 8 || off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off:]))
	off += 2
	for k := 0; k < n; k++ {
		e := off + k*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) != 0x0112 {
			continue
		}
		if bo.Uint16(t[e+2:]) != 3 || bo.Uint32(t[e+4:]) != 1 {
			return 1
		}
		if v := int(bo.Uint16(t[e+8:])); v >= 1 && v <= 8 {
			return v
		}
		return 1
	}
	return 1
}
