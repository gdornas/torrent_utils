package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	//"regexp"
	//"sort"
	"strconv"
	"strings"
	"sync"
)

type argsStruct struct {

	name	string
	fileSearch	bool

	minSize int
	maxSize int
	minHits int
	maxHits int
	minFiles int
	maxFiles int

	minFirstSeen string
	maxFirstSeen string
	minLastSeen string
	maxLastSeen string

	sortHits bool
	sortName bool
	sortSize bool
	sortFiles bool
	sortFirstSeen bool
	sortLastSeen bool
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

	flag.StringVar(&args.name, "n", "gentoo", "string to be searched for in torrent name")
	flag.BoolVar(&args.fileSearch, "N", false, "search also in filenames")

	flag.IntVar(&args.minSize, "s", 0, "min size in MB")
	flag.IntVar(&args.minSize, "S", 1073741824, "max size in MB")
	flag.IntVar(&args.minHits, "h", 0, "min number of hits")
	flag.IntVar(&args.maxHits, "H", 1073741824, "max number of hits")
	flag.IntVar(&args.minFiles, "f", 0, "min number of files")
	flag.IntVar(&args.maxFiles, "F", 1073741824, "max number of files")

	flag.StringVar(&args.minFirstSeen, "d", "1970-01-01", "min first seen date: YYYY-MM-DD")
	flag.StringVar(&args.maxFirstSeen, "D", "2100-01-01", "max first seen date: YYYY-MM-DD")
	flag.StringVar(&args.minLastSeen, "l", "1970-01-01", "min last seen date: YYYY-MM-DD")
	flag.StringVar(&args.maxLastSeen, "L", "2100-01-01", "max last seen date: YYYY-MM-DD")

	flag.BoolVar(&args.sortHits, "1", false, "")
	flag.BoolVar(&args.sortName, "2", false, "")
	flag.BoolVar(&args.sortSize, "3", false, "")
	flag.BoolVar(&args.sortFiles, "4", false, "")
	flag.BoolVar(&args.sortFirstSeen, "5", false, "")
	flag.BoolVar(&args.sortLastSeen, "6", false, "")
}

func main() {

	flag.Usage = printUsage
	flag.Parse()

	file, err := os.Open(os.Args[1])
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
	fmt.Println(len(resultList))
}

func filterLines(lines <-chan string, results chan<- lineStruct, wg *sync.WaitGroup) {

	defer wg.Done()

	for line := range lines {

		l := parseLine(line)

		if l.hits > 8 {
			results <- l
		}
	}
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

func errExit(err error) {

	if err != nil {
		log.Fatal(err)
	}
}

func printUsage() {

	fmt.Printf("Usage: %s [options] <file.torrent>\n", os.Args[0])
	flag.PrintDefaults()
}
