package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	//"sort"
	"strconv"
	"strings"
	"sync"
)

type argsStruct struct {
	name       string
	fileSearch bool
	unordered  bool
	any        bool
	exact      bool

	minSize  int
	maxSize  int
	minHits  int
	maxHits  int
	minFiles int
	maxFiles int

	minFirstSeen string
	maxFirstSeen string
	minLastSeen  string
	maxLastSeen  string

	sortName      bool
	sortSize      bool
	sortFiles     bool
	sortFirstSeen bool
	sortLastSeen  bool
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

var args argsStruct
var workers int = 64

func init() {

	flag.StringVar(&args.name, "n", "gentoo", "")
	flag.BoolVar(&args.fileSearch, "N", false, "")
	flag.BoolVar(&args.unordered, "u", false, "")
	flag.BoolVar(&args.any, "a", false, "")
	flag.BoolVar(&args.exact, "r", false, "")

	flag.IntVar(&args.minSize, "s", 0, "")
	flag.IntVar(&args.maxSize, "S", 999999999999, "")
	flag.IntVar(&args.minHits, "p", 0, "")
	flag.IntVar(&args.maxHits, "P", 999999999999, "")
	flag.IntVar(&args.minFiles, "f", 0, "")
	flag.IntVar(&args.maxFiles, "F", 999999999999, "")

	flag.StringVar(&args.minFirstSeen, "d", "1970-01-01", "")
	flag.StringVar(&args.maxFirstSeen, "D", "2100-01-01", "")
	flag.StringVar(&args.minLastSeen, "l", "1970-01-01", "")
	flag.StringVar(&args.maxLastSeen, "L", "2100-01-01", "")

	flag.BoolVar(&args.sortName, "1", false, "")
	flag.BoolVar(&args.sortSize, "2", false, "")
	flag.BoolVar(&args.sortFiles, "3", false, "")
	flag.BoolVar(&args.sortFirstSeen, "4", false, "")
	flag.BoolVar(&args.sortLastSeen, "5", false, "")
}

func main() {

	flag.Usage = printUsage
	flag.Parse()

	file, err := os.Open(flag.Arg(0))
	errExit(err)
	defer file.Close()

	lines := make(chan string)
	results := make(chan lineStruct)

	wg := new(sync.WaitGroup)

	for w := 1; w <= workers; w++ {
		wg.Add(1)
		go filterLines(lines, results, wg)
	}

	go func() {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var resultList []lineStruct

	for res := range results {
		resultList = append(resultList, res)
	}

	fmt.Println("Found results:", len(resultList))
	for _, line := range resultList {
		printLine(line)
	}
}

func filterLines(lines <-chan string, results chan<- lineStruct, wg *sync.WaitGroup) {

	defer wg.Done()

	for line := range lines {

		l := parseLine(line)
		l.size /= (1024 * 1024)

		if skipName(l) || skipNumOrDate(l) {
			continue
		}

		results <- l
	}
}

func skipName(l lineStruct) bool {

	if args.unordered {
		return noNameUnsorted(l)

	} else if args.any {
		return noNameAny(l)

	} else if args.exact {
		return noNameRegexp(l)

	} else {
		return noNameDefault(l)
	}
}

func noNameUnsorted(l lineStruct) bool {

	lowerName := strings.ToLower(l.name)
	searchStr := strings.ToLower(args.name)
	words := strings.Split(searchStr, " ")

	for _, word := range words {

		if !strings.Contains(lowerName, word) {
			return true
		}
	}

	return false
}

func noNameAny(l lineStruct) bool {

	lowerName := strings.ToLower(l.name)
	searchStr := strings.ToLower(args.name)
	words := strings.Split(searchStr, " ")

	for _, word := range words {

		if strings.Contains(lowerName, word) {
			return false
		}
	}

	return true
}

func noNameRegexp(l lineStruct) bool {

	re, err := regexp.Compile(args.name)
	errExit(err)

	if !re.MatchString(l.name) {
		return true
	}

	return false
}

func noNameDefault(l lineStruct) bool {

	lowerName := strings.ToLower(l.name)
	searchStr := strings.ToLower(args.name)
	words := strings.Split(searchStr, " ")

	for _, word := range words {

		lowerNameSlice := strings.Split(lowerName, word)

		if len(lowerNameSlice) == 1 {
			return true
		} else {
			lowerName = lowerNameSlice[1]
		}
	}

	return false
}

func skipNumOrDate(l lineStruct) bool {

	if l.size > args.maxSize || l.size < args.minSize {
		return true
	}

	if l.hits > args.maxHits || l.hits < args.minHits {
		return true
	}

	if l.files > args.maxFiles || l.files < args.minFiles {
		return true
	}

	if l.firstSeen > args.maxFirstSeen || l.firstSeen < args.minFirstSeen {
		return true
	}

	if l.lastSeen > args.maxLastSeen || l.lastSeen < args.minLastSeen {
		return true
	}

	return false
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

func printLine(line lineStruct) {

	fmt.Printf("%s\t%14d\t%11d\t%s\t%s\t%5d\t%s\n",
		line.hash,
		line.size,
		line.files,
		line.firstSeen,
		line.lastSeen,
		line.hits,
		line.name)
}

func errExit(err error) {

	if err != nil {
		log.Fatal(err)
	}
}

func printUsage() {

	fmt.Printf(`
usage: %s [options] <torrentdb directory>

search string options:
	-n	ordered words to be searched for in torrent names
	-N	toggle searching also in the filenames
	-u	toggle search of unordered words in search string
	-a	toggle search of any word in search string
	-r	toggle regexp in search string, case sensitive

numeric filters:
	-s	min size in MB
	-S	max size in MB
	-p	min number of hits (p like popularity)
	-P	max number of hits (P like popularity)
	-f	min number of files
	-F	max number of files

date filters (format YYYY-MM-DD):
	-d	min first seen date
	-D	max first seen date
	-l	min last seen date
	-L	max last seen date

sorting options (default is by hits)
	-1	by names
	-2	by size
	-3	by number of files
	-4	by first seen
	-5	by last seen

`, os.Args[0])
}
