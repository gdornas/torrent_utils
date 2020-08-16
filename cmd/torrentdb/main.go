package main

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tp "github.com/torrentdb/torrent_utils/lib/torrentparse"
)

// TODO
// modify torrents.tsv in-place (don't overwrite them)

type argsStruct struct {
	tordir *string
	dbdir  *string
}

type lineStruct struct {
	hash      string
	size      int
	files     int
	firstSeen string
	lastSeen  string
	hits      int
	name      string
}

type statsStruct struct {
	scanTime      int64
	lastScanTime  int64
	countNew      int
	countUpdated  int
	countFiles    int
	countPreTotal int
	countRejected int
}

var args argsStruct
var stats statsStruct

func init() {

	args.tordir = flag.String("t", "", "dir with torsniff dir structure")
	args.dbdir = flag.String("d", "", "database dir")
}

func main() {

	flag.Usage = printUsage
	flag.Parse()

	stats.lastScanTime = getLastScan()
	dt := time.Unix(stats.lastScanTime, 0)
	fmt.Println("* last scan:", dt.Format("2006-01-02 15:04"))

	fmt.Println("* finding all torrent files in the directory...")
	stats.scanTime = time.Now().Unix()
	torrentFiles, err := filepath.Glob(*args.tordir + "/*/*/*.torrent")
	errExit(err)

	stats.countFiles = len(torrentFiles)

	fmt.Println("* loading torrents.tsv into memory...")
	torrents, hashList := loadTorrents()

	fmt.Println("* parsing torrent files, updating the buffers and dumping torrent files...")
	newTorrents, updatedHashList := processFiles(torrentFiles, hashList)
	sort.Strings(updatedHashList)

	stats.countNew = len(newTorrents)
	stats.countUpdated = len(updatedHashList)
	stats.countPreTotal = len(torrents)

	fmt.Println("* dumping torrent.tsv...")
	dumpTorrents(torrents, newTorrents, updatedHashList)
	dumpStats()
}

func processFiles(torrentFiles []string, hashList []string) ([]string, []string) {

	var newTorrents []string
	var updatedHashList []string

	filesFile := *args.dbdir + "/files.tsv"
	fFiles, err := os.OpenFile(filesFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	defer fFiles.Close()
	errExit(err)


	for _, torrentFile := range torrentFiles {

		stat, _ := os.Stat(torrentFile)
		mTime := stat.ModTime().Unix()
		if mTime < stats.lastScanTime || mTime > stats.scanTime {
			continue
		}

		f, err := os.Open(torrentFile)
		errExit(err)
		t, err := tp.ParseTorrent(f)
		f.Close()
		if err != nil {
			stats.countRejected++
			logParseError(f.Name(), err)
			continue
		}

		hash := hex.EncodeToString(t.Hash[:])

		err = torrentIsValid(t)
		if err != nil {
			stats.countRejected++
			logParseError(hash, err)
			continue
		}

		hashID := sort.SearchStrings(hashList, hash)

		if hashID > len(hashList)-1 || hashList[hashID] != hash {
			line := torrentToLine(t)
			newTorrents = append(newTorrents, printLine(line))
			dumpTFiles(fFiles, line, t)
		} else {
			updatedHashList = append(updatedHashList, hashList[hashID])
		}
	}

	return newTorrents, updatedHashList
}

func getLastScan() int64 {

	lastScanFile := *args.dbdir + "/stats.txt"

	if !pathExists(lastScanFile) {

		return 0
	}

	f, err := os.Open(lastScanFile)
	defer f.Close()
	errExit(err)

	var lastScan int64
	var l string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {

		l = scanner.Text()
	}

	lastScanStr := strings.Split(l, "\t")[0]
	lastScan, err = strconv.ParseInt(lastScanStr, 10, 64)
	errExit(err)

	return lastScan
}

func loadTorrents() ([]string, []string) {

	var torrents []string
	var hashList []string

	inFile := *args.dbdir + "/torrents.tsv"

	if !pathExists(inFile) {

		f, err := os.OpenFile(inFile, os.O_CREATE|os.O_RDONLY, 0644)
		f.Close()
		errExit(err)
	}

	f, err := os.Open(inFile)
	defer f.Close()
	errExit(err)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {

		l := scanner.Text()
		hash := strings.Split(l, "\t")[0]
		torrents = append(torrents, l)
		hashList = append(hashList, hash)
	}

	sort.Strings(hashList)

	return torrents, hashList
}

func parseLine(l string) lineStruct {

	var line lineStruct
	var err error

	ll := strings.Split(l, "\t")

	line.hash = ll[0]

	line.size, err = strconv.Atoi(strings.TrimSpace(ll[1]))
	errExit(err)

	line.files, err = strconv.Atoi(strings.TrimSpace(ll[2]))
	errExit(err)

	line.firstSeen = strings.TrimSpace(ll[3])
	line.lastSeen = strings.TrimSpace(ll[4])

	line.hits, err = strconv.Atoi(strings.TrimSpace(ll[5]))
	errExit(err)

	line.name = strings.TrimSpace(ll[6])

	return line
}

func torrentToLine(t *tp.Info) lineStruct {

	var line lineStruct
	dt := time.Now()
	today := dt.Format("2006-01-02")

	line.hash = hex.EncodeToString(t.Hash[:])
	line.size = int(t.Length)
	line.files = len(t.Files)
	line.firstSeen = today
	line.lastSeen = today
	line.hits = 1
	line.name = t.Name

	return line
}

func printLine(line lineStruct) string {

	l := fmt.Sprintf("%s\t%14d\t%11d\t%s\t%s\t%5d\t%s",
		line.hash,
		line.size,
		line.files,
		line.firstSeen,
		line.lastSeen,
		line.hits,
		line.name)

	return l
}

func torrentIsValid(t *tp.Info) error {

	if !utf8.Valid([]byte(t.Name)) {

		return fmt.Errorf("non UTF-8 char in name: %s", t.Name)
	}

	isAllowed, c, i := stringIsAllowed(t.Name)
	if !isAllowed {

		return fmt.Errorf("not allowed char %d: 0x%0.2x %U; in name: %s",
			i+1, c, c, t.Name)
	}

	for _, f := range t.Files {

		if !utf8.Valid([]byte(f.Path)) {

			return fmt.Errorf("non UTF-8 char in filename: %s", f.Path)
		}

		isAllowed, c, i := stringIsAllowed(f.Path)
		if !isAllowed {

			return fmt.Errorf("not allowed char %d: 0x%0.2x %U; in filename: %s",
				i+1, c, c, f.Path)
		}
	}

	return nil
}

func stringIsAllowed(s string) (bool, rune, int) {

	for i, c := range s {

		if unicode.IsControl(c) {
			return false, c, i
		}
	}

	return true, '0', 0
}

func dumpTorrents(torrents []string, newTorrents []string, updatedHashList []string) {

	torrentsFile := *args.dbdir + "/torrents.tsv"
	fTorrents, err := os.OpenFile(torrentsFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	defer fTorrents.Close()
	errExit(err)

	dt := time.Now()
	today := dt.Format("2006-01-02")

	for _, l := range torrents {

		line := parseLine(l)
		hashID := sort.SearchStrings(updatedHashList, line.hash)

		if hashID > len(updatedHashList)-1 || updatedHashList[hashID] != line.hash {
			fmt.Fprintln(fTorrents, l)
		} else {
			line.hits++
			line.lastSeen = today
			l = printLine(line)
			fmt.Fprintln(fTorrents, l)
		}
	}

	for _, l := range newTorrents {
		fmt.Fprintln(fTorrents, l)
	}
}

func dumpTFiles(fFiles *os.File, line lineStruct, t *tp.Info) {

	fmt.Fprintln(fFiles, "hash:", line.hash)
	for _, tFile := range t.Files {

		fmt.Fprintf(fFiles, "%d\t%s\n", tFile.Length, tFile.Path)
	}
	fmt.Fprintln(fFiles, "---")
}

func dumpStats() {

	statsFile := *args.dbdir + "/stats.txt"

	if !pathExists(statsFile) {

		f, err := os.OpenFile(statsFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		defer f.Close()
		errExit(err)

		fmt.Fprintf(f, "%s\t%s\t%18s\t%10s\t%10s\t%10s\t%10s\t%10s\n",
			"scan unixtime",
			"scan datetime",
			"new",
			"updated",
			"rejected",
			"processed",
			"files",
			"db total")
	}

	f, err := os.OpenFile(statsFile, os.O_APPEND|os.O_WRONLY, 0644)
	defer f.Close()
	errExit(err)

	scanTimeTime := time.Unix(stats.scanTime, 0)

	fmt.Fprintf(f, "%d\t%s\t%10d\t%10d\t%10d\t%10d\t%10d\t%10d\n",
		scanTimeTime.Unix(),
		scanTimeTime.Format("2006-01-02 15:04"),
		stats.countNew,
		stats.countUpdated,
		stats.countRejected,
		stats.countNew+stats.countUpdated,
		stats.countFiles,
		stats.countPreTotal+stats.countNew)
}

func pathExists(path string) bool {

	_, err := os.Stat(path)
	if err == nil {
		return true
	}

	if os.IsNotExist(err) {
		return false
	}

	errExit(err)

	return false
}

func printUsage() {

	fmt.Printf("Usage: %s [options] <file.torrent>\n", os.Args[0])
	flag.PrintDefaults()
}

func errExit(err error) {

	if err != nil {
		fmt.Println("error")
		log.Fatal(err)
	}
}

func logParseError(logmsg string, logerr error) {

	errorFile := *args.dbdir + "/error.log"
	f, err := os.OpenFile(errorFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	errExit(err)
	defer f.Close()

	log.SetOutput(f)
	log.Println(logmsg, logerr)
}


// FUCK YOU. hahahahahaha!
