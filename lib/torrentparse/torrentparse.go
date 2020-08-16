package torrentparse

import (
	"crypto/sha1" // nolint: gosec
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/zeebo/bencode"
)

// information about torrent from metainfo dictionary
type Info struct {
	PieceLength  uint32
	Name         string
	Hash         [20]byte
	HashInt      uint64
	HashStr      string
	HashStrShort string
	Length       int64
	NumPieces    uint32
	Bytes        []byte
	Files        []File
	FilesNo      int
	pieces       []byte
}

// files inside a torrent
type File struct {
	Length int64
	Path   string
}

type file struct {
	Length int64    `bencode:"length"`
	Path   []string `bencode:"path"`
}

func ParseTorrent(r io.Reader) (*Info, error) {

	var metaInfo struct {
		Info bencode.RawMessage `bencode:"info"`
	}

	err := bencode.NewDecoder(r).Decode(&metaInfo)
	if err != nil {
		return nil, fmt.Errorf("Error when decoding metainfo dictionary.")
	}

	if len(metaInfo.Info) == 0 {
		return nil, fmt.Errorf("No info dict in torrent file.")
	}

	info, err := ParseInfo(metaInfo.Info)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func ParseInfo(b []byte) (*Info, error) {

	var ib struct {
		PieceLength uint32 `bencode:"piece length"`
		Pieces      []byte `bencode:"pieces"`
		Name        string `bencode:"name"`
		Length      int64  `bencode:"length"` // Single File Mode
		Files       []file `bencode:"files"`  // Multiple File mode
	}

	if err := bencode.DecodeBytes(b, &ib); err != nil {
		return nil, fmt.Errorf("Error when decoding info dictionary.")
	}

	if ib.PieceLength == 0 {
		return nil, fmt.Errorf("Torrent has zero piece length.")
	}

	if len(ib.Pieces)%sha1.Size != 0 {
		return nil, fmt.Errorf("Invalid piece data.")
	}

	numPieces := len(ib.Pieces) / sha1.Size
	if numPieces == 0 {
		return nil, fmt.Errorf("Torrent has zero pieces.")
	}

	if err := validateFilenames(ib.Files); err != nil {
		return nil, err
	}

	i := Info{
		PieceLength: ib.PieceLength,
		NumPieces:   uint32(numPieces),
		pieces:      ib.Pieces,
		Name:        ib.Name,
	}

	multiFile := len(ib.Files) > 0
	if multiFile {
		for _, f := range ib.Files {
			i.Length += f.Length
		}
		parseMultiFiles(&i, ib.Files)
	} else {
		i.Length = ib.Length
		i.Files = []File{{Path: cleanName(i.Name), Length: i.Length}}
	}
	i.FilesNo = len(i.Files)

	totalPieceDataLength := int64(i.PieceLength) * int64(i.NumPieces)
	delta := totalPieceDataLength - i.Length
	if delta >= int64(i.PieceLength) || delta < 0 {
		return nil, fmt.Errorf("Invalid piece length.")
	}

	i.Bytes = b
	calcHash(&i)

	if ib.Name == "" {
		i.Name = "__empty_name_field_in_info_dict__"
	} else {
		i.Name = ib.Name
	}

	return &i, nil
}

func cleanName(s string) string {

	s = strings.ToValidUTF8(s, string(unicode.ReplacementChar))
	s = strings.ToValidUTF8(s, "")
	return s
}

func errExit(err error) {

	if err != nil {
		log.Fatal(err)
	}
}

func validateFilenames(files []file) error {

	for _, file := range files {

		// ".." is not allowed in file names
		for _, pathPart := range file.Path {
			pathPart = strings.TrimSpace(pathPart)
			if pathPart == ".." {
				return fmt.Errorf("invalid file name: %q",
					filepath.Join(file.Path...))
			}
		}
	}
	return nil
}

func parseMultiFiles(i *Info, files []file) {

	i.Files = make([]File, len(files))
	for j, f := range files {
		parts := make([]string, 0, len(f.Path)+1)
		parts = append(parts, cleanName(i.Name))
		for _, p := range f.Path {
			parts = append(parts, cleanName(p))
		}
		i.Files[j] = File{
			Path:   filepath.Join(parts...),
			Length: f.Length,
		}
	}
}

func calcHash(i *Info) {

	hash := sha1.New()         // nolint: gosec
	_, _ = hash.Write(i.Bytes) // nolint: gosec
	copy(i.Hash[:], hash.Sum(nil))
}
