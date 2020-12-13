package main

import (
	//"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/anacrolix/torrent/bencode"
)

type argsStruct struct {
	seeders    int
	downloaded int
	leechers   int
	terse      bool
}

type scrapeItem struct {
	Seeders    int `bencode:"complete"`
	Downloaded int `bencode:"downloaded"`
	Leechers   int `bencode:"incomplete"`
}

type scrapeResult struct {
	Files map[string]scrapeItem `bencode:"files"`
}

type statsStruct struct {
	zero           int
	band_1_9       int
	band_10_99     int
	band_100_999   int
	band_1000_9999 int
	band_10000_inf int
	val	       int
}

var args argsStruct
var totalItems int

func init() {

	flag.IntVar(&args.seeders, "s", 0, "")
	flag.IntVar(&args.downloaded, "d", 0, "")
	flag.IntVar(&args.leechers, "l", 0, "")
	flag.BoolVar(&args.terse, "t", false, "")
}

func main() {

	flag.Usage = printUsage
	flag.Parse()

	inputFile := flag.Arg(0)
	inF, err := ioutil.ReadFile(inputFile)
	errExit(err)

	var scrape scrapeResult
	err = bencode.Unmarshal(inF, &scrape)
	errExit(err)

	outFile := inputFile + ".decoded.txt"
	outF, err := os.OpenFile(outFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	defer outF.Close()
	errExit(err)

	// print header
	if !args.terse {
		fmt.Fprintf(outF, "%s\t\t\t\t\t %s      %s\t%s\n",
			"info hash", "seeders", "downloaded", "leechers")
	}

	var seedStats statsStruct
	var downStats statsStruct
	var leechStats statsStruct

	// filter and print scrape entries
	for infoHash, item := range scrape.Files {

		infoHash = hex.EncodeToString([]byte(infoHash))

		updateStats(infoHash, item, &seedStats, "seeders")
		updateStats(infoHash, item, &downStats, "downloaded")
		updateStats(infoHash, item, &leechStats, "leechers")

		if item.Seeders < args.seeders {
			continue
		}

		if item.Downloaded < args.downloaded {
			continue
		}

		if item.Leechers < args.leechers {
			continue
		}

		if infoHash == "0000000000000000000000000000000000000000" {
			continue
		}

		printEntry(outF, infoHash, item)
	}

	// dump stats to stdout
	output := fmt.Sprintf("%v %v %v %d", seedStats, downStats, leechStats, totalItems)
	output = strings.Replace(output, "{", "", -1)
	output = strings.Replace(output, "}", "", -1)
	output = strings.Replace(output, " ", "\t", -1)
	date := time.Now().Format("2006-01-02 15:04")

	fmt.Printf("%s\t%s\n", date, output)
}

func updateStats(infoHash string, item scrapeItem, stats *statsStruct, statType string) {

	var val int

	switch statType {
	case "seeders":
		val = item.Seeders
		totalItems++
	case "downloaded":
		val = item.Downloaded
	case "leechers":
		val = item.Leechers
	}

	stats.val += val

	if val == 0 {
		stats.zero++
	} else if val >= 1 && val <= 9 {
		stats.band_1_9++
	} else if val >= 10 && val <= 99 {
		stats.band_10_99++
	} else if val >= 100 && val <= 999 {
		stats.band_100_999++
	} else if val >= 1000 && val <= 9999 {
		stats.band_1000_9999++
	} else if val >= 10000 {
		stats.band_10000_inf++
	}
}

func printEntry(outF *os.File, infoHash string, item scrapeItem) {

	if args.terse {
		fmt.Fprintln(outF, infoHash)
	} else {
		fmt.Fprintf(outF, "%s\t%8d\t%8d\t%8d\n",
			infoHash,
			item.Seeders,
			item.Downloaded,
			item.Leechers)
	}
}

func errExit(err error) {

	if err != nil {
		log.Fatal(err)
	}
}

func printUsage() {

	fmt.Printf(`
usage: %s [options] <torrent scrape file>

options:
	-s	min number of seeders to include in output
	-l	min number of leechers to include in output
	-d	min number of downloaded to include in output
	-t	terse output, dump just hashses without torrent stats

`, os.Args[0])
}
