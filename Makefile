build:
	go build -o bin/torrentparse cmd/torrentparse/main.go
	go build -o bin/torrentdb cmd/torrentdb/*

test: build
	./test/torrentdb.sh

# this requires data outside of this repo
test_big: build
	./test/torrentdb_big.sh
