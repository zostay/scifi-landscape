// Package scenefile reads and writes scene files: ordinary PNG images carrying
// the data needed to reproduce the scene, embedded as PNG tEXt chunks.
//
// A scene file is a normal PNG — any viewer shows the picture — with extra
// textual metadata under the "scifi-landscape/" key prefix: the seed, and the
// complete config, globals, and scene list as YAML. Given a scene file the app
// can reproduce the scene from any layer (seed, config, globals, or scene list).
//
// The standard library's image/png cannot write tEXt chunks, so this package
// splices them into the encoded PNG itself. It depends only on the standard
// library, keeping the format simple and stable.
package scenefile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"os"
	"sort"
)

// Text-chunk keywords embedded in a scene file. These are the reproducibility
// layers: the seed (decimal), and the complete config, globals, and scene list as
// YAML.
const (
	KeySeed      = "scifi-landscape/seed"
	KeyConfig    = "scifi-landscape/config.yaml"
	KeyGlobals   = "scifi-landscape/globals.yaml"
	KeySceneList = "scifi-landscape/scene-list.yaml"
)

// orderedKeys lists the known scene-file keys in the order they are written, so a
// scene file's chunks come out in a stable, meaningful order (seed first, then
// each successively-derived layer).
var orderedKeys = []string{KeySeed, KeyConfig, KeyGlobals, KeySceneList}

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// Write encodes img as a PNG and embeds texts as tEXt chunks, producing a scene
// file. Keys are written in the canonical scene-file order (seed, config,
// globals, scene list); any other keys follow in sorted order. Empty-valued
// entries are skipped. A keyword must be 1–79 bytes and contain no NUL; text must
// contain no NUL.
func Write(w io.Writer, img image.Image, texts map[string]string) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return fmt.Errorf("scenefile: encode: %w", err)
	}
	pngBytes := buf.Bytes()

	// The PNG ends with a fixed 12-byte IEND chunk; insert the tEXt chunks just
	// before it.
	if len(pngBytes) < 12 {
		return fmt.Errorf("scenefile: encoded PNG too short")
	}
	body, iend := pngBytes[:len(pngBytes)-12], pngBytes[len(pngBytes)-12:]

	if _, err := w.Write(body); err != nil {
		return err
	}
	for _, k := range writeOrder(texts) {
		text := texts[k]
		if text == "" {
			continue
		}
		chunk, err := textChunk(k, text)
		if err != nil {
			return err
		}
		if _, err := w.Write(chunk); err != nil {
			return err
		}
	}
	_, err := w.Write(iend)
	return err
}

// writeOrder returns the keys of texts: the known keys first in canonical order,
// then any remaining keys sorted, so output is deterministic.
func writeOrder(texts map[string]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, k := range orderedKeys {
		if _, ok := texts[k]; ok {
			out = append(out, k)
			seen[k] = true
		}
	}
	var extra []string
	for k := range texts {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	return append(out, extra...)
}

// textChunk builds a PNG tEXt chunk for the given keyword and text.
func textChunk(keyword, text string) ([]byte, error) {
	if len(keyword) < 1 || len(keyword) > 79 {
		return nil, fmt.Errorf("scenefile: keyword %q must be 1-79 bytes", keyword)
	}
	if bytes.IndexByte([]byte(keyword), 0) >= 0 || bytes.IndexByte([]byte(text), 0) >= 0 {
		return nil, fmt.Errorf("scenefile: keyword/text must not contain NUL")
	}
	data := make([]byte, 0, len(keyword)+1+len(text))
	data = append(data, keyword...)
	data = append(data, 0)
	data = append(data, text...)
	return rawChunk("tEXt", data), nil
}

// rawChunk assembles a PNG chunk: length, type, data, CRC32(type+data).
func rawChunk(typ string, data []byte) []byte {
	out := make([]byte, 0, 12+len(data))
	out = binary.BigEndian.AppendUint32(out, uint32(len(data)))
	out = append(out, typ...)
	out = append(out, data...)
	crc := crc32.NewIEEE()
	crc.Write([]byte(typ))
	crc.Write(data)
	out = binary.BigEndian.AppendUint32(out, crc.Sum32())
	return out
}

// ReadTexts returns the tEXt chunks of a scene file as a keyword→text map,
// without decoding the image. A plain PNG with no tEXt chunks yields an empty
// map. It errors if the data is not a valid PNG.
func ReadTexts(r io.Reader) (map[string]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	texts := map[string]string{}
	err = walkChunks(data, func(typ string, chunk []byte) error {
		if typ != "tEXt" {
			return nil
		}
		i := bytes.IndexByte(chunk, 0)
		if i < 0 {
			return fmt.Errorf("scenefile: tEXt chunk missing NUL separator")
		}
		texts[string(chunk[:i])] = string(chunk[i+1:])
		return nil
	})
	if err != nil {
		return nil, err
	}
	return texts, nil
}

// ReadTextsFile reads the tEXt chunks from the scene file at path, a convenience
// for callers that have a filename rather than a reader.
func ReadTextsFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadTexts(f)
}

// Read returns both the decoded image and the tEXt chunks of a scene file.
func Read(r io.Reader) (image.Image, map[string]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}
	texts, err := ReadTexts(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("scenefile: decode: %w", err)
	}
	return img, texts, nil
}

// walkChunks iterates the chunks of a PNG byte stream, calling fn with each
// chunk's type and data. It validates the signature and chunk framing.
func walkChunks(data []byte, fn func(typ string, chunk []byte) error) error {
	if len(data) < len(pngSignature) || !bytes.Equal(data[:len(pngSignature)], pngSignature) {
		return fmt.Errorf("scenefile: not a PNG")
	}
	pos := len(pngSignature)
	for pos+8 <= len(data) {
		length := binary.BigEndian.Uint32(data[pos : pos+4])
		typ := string(data[pos+4 : pos+8])
		start := pos + 8
		end := start + int(length)
		if end+4 > len(data) {
			return fmt.Errorf("scenefile: truncated %s chunk", typ)
		}
		if err := fn(typ, data[start:end]); err != nil {
			return err
		}
		pos = end + 4 // skip the 4-byte CRC
		if typ == "IEND" {
			break
		}
	}
	return nil
}
