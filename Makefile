build:
	go build -o bin/torrentparse cmd/torrentparse/main.go
	go build -o bin/torrentdb cmd/torrentdb/*
	go build -o bin/torrentdbq cmd/torrentdbq/*
	go build -o bin/scrapedump cmd/scrapedump/*

test: build
	./test/torrentdb.sh

# this requires data outside of this repo
test_big: build
	./test/torrentdb_big.sh
