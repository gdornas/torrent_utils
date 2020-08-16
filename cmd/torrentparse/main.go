package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"

	tp "github.com/torrentdb/torrent_utils/lib/torrentparse"
)

type args_s struct {
	tfile   string
	verbose *bool
}

var args args_s

func init() {

	args.verbose = flag.Bool("v", false, "Print more info on torrent files")
}

func main() {

	flag.Usage = printUsage
	flag.Parse()
	args.tfile = flag.Arg(0)

	f, err := os.Open(args.tfile)
	errExit(err)

	t, err := tp.ParseTorrent(f)
	errExit(err)

	printInfo(t)
}

func printUsage() {

	fmt.Printf("Usage: %s [options] <file.torrent>\n", os.Args[0])
	flag.PrintDefaults()
}

func printInfo(t *tp.Info) {

	fmt.Print("\nName\t\t", t.Name, "\n",
		"Hash\t\t", hex.EncodeToString(t.Hash[:]), "\n",
		"Files\t\t", t.FilesNo, "\n",
		"Size(MB)\t", t.Length/1024/1024, "\n",
	)

	if *args.verbose {

		fmt.Print("NumPieces\t", t.NumPieces, "\n",
			"PieceSize(MB)\t", t.PieceLength, "\n\n",
		)

		for i, file := range t.Files {
			fmt.Print("File", i, "\t\t", file.Path, "\n")
			fmt.Print("Size", i, "(MB)\t",
				file.Length/1024/1024, "\n")
		}
	}
	fmt.Println()
}

func errExit(err error) {

	if err != nil {
		log.Fatal(err)
	}
}
