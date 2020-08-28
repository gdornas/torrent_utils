package main

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
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
	hashList, indexList := loadTorrents()

	fmt.Println("* parsing torrent files, updating the buffers and dumping torrent files...")
	processFiles(torrentFiles, hashList, indexList)

	stats.countPreTotal = len(hashList)

	fmt.Println("* dumping torrent.tsv...")
	//dumpTorrents(torrents, newTorrents )
	dumpStats()
}

func processFiles(torrentFiles []string, hashList []string, indexList []int64) {

	newTorrentsCheck := make(map[string]bool)
	var newTorrents []string

	torrentsFile := *args.dbdir + "/torrents.tsv"
	fTorrents, err := os.OpenFile(torrentsFile, os.O_CREATE|os.O_RDWR, 0644)
	defer fTorrents.Close()
	errExit(err)

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

			line := torrentToLine(t, stat)

			// skip new hashes from wrongly named torrent files
			if _, exists := newTorrentsCheck[hash]; exists {
				continue
			}

			newTorrents = append(newTorrents, printLine(line))
			newTorrentsCheck[hash] = true

			dumpTFiles(fFiles, line, t)
			stats.countNew++
		} else {
			updateLine(fTorrents, indexList, hash, hashID, stat)
		}
	}

	_, err = fTorrents.Seek(0, 2)
	errExit(err)
	for _, l := range newTorrents {
		fmt.Fprintln(fTorrents, l)
	}

	return
}

func updateLine(fTorrents *os.File, indexList []int64, hash string, hashID int,
	stat os.FileInfo) {

	// get and modify the line
	_, err := fTorrents.Seek(indexList[hashID], 0)
	errExit(err)

	var line lineStruct
	var partName string
	_, err = fmt.Fscan(fTorrents,
		&line.hash, &line.size, &line.files,
		&line.firstSeen, &line.lastSeen, &line.hits,
		&partName)
	errExit(err)

	line.hits++
	line.lastSeen = stat.ModTime().Format("2006-01-02")

	if line.hash != hash {
		errExit(fmt.Errorf("modifing incorrect hash: %s - %s",
			hash, line.hash))
	}

	// update then line
	_, err = fTorrents.Seek(indexList[hashID], 0)
	fmt.Fprint(fTorrents, printLine(line))

	// check the line
	_, err = fTorrents.Seek(indexList[hashID], 0)
	errExit(err)

	var lineMod lineStruct
	var partNameMod string
	_, err = fmt.Fscan(fTorrents,
		&lineMod.hash, &lineMod.size, &lineMod.files,
		&lineMod.firstSeen, &lineMod.lastSeen, &lineMod.hits,
		&partNameMod)
	errExit(err)

	if !reflect.DeepEqual(line, lineMod) || partName != partNameMod {
		errExit(fmt.Errorf("writing hash failed: %s - %s",
			hash, line.hash))
	}

	stats.countUpdated++
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

func loadTorrents() ([]string, []int64) {

	var hashList []string
	var indexList []int64
	var hashIndexList []string
	var lineIndex int64

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
		iStr := strconv.FormatInt(lineIndex, 10)

		hashIndex := strings.Split(l, "\t")[0] + "\t" + iStr
		hashIndexList = append(hashIndexList, hashIndex)
		lineIndex += int64(len(l)) + 1
	}

	sort.Strings(hashIndexList)

	for _, hashIndex := range hashIndexList {

		hi := strings.Split(hashIndex, "\t")
		hash := hi[0]
		index, err := strconv.ParseInt(hi[1], 10, 64)
		errExit(err)

		if len(hash) != 40 || index < 0 {
			errExit(fmt.Errorf("incorrect hash and index: %s - %s",
				hash, index))
		}

		hashList = append(hashList, hash)
		indexList = append(indexList, index)
	}

	return hashList, indexList
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

func torrentToLine(t *tp.Info, stat os.FileInfo) lineStruct {

	var line lineStruct
	mtime := stat.ModTime().Format("2006-01-02")

	line.hash = hex.EncodeToString(t.Hash[:])
	line.size = int(t.Length)
	line.files = len(t.Files)
	line.firstSeen = mtime
	line.lastSeen = mtime
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
