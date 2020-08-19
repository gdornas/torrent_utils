#!/bin/ksh

set -eu

rm -r test/tmp
mkdir -p test/tmp/torrentdb
find test/torsniff_dump.1 -type f -name '*.torrent' \
	-exec touch -m -d '2002-05-14' {} \+
find test/torsniff_dump.2 -type f -name '*.torrent' \
	-exec touch {} \+
touch -m -d '2007-08-15' test/torsniff_dump.2/00/ff/038de228*d21010aa4.torrent



echo '\n*** pass 1 - preparing db\n'

./bin/torrentdb -d test/tmp/torrentdb -t test/torsniff_dump.1



echo '\n*** pass 1 - testing the output\n'

cp -a test/bench.1 test/tmp

awk -i inplace -F'\t' '{ $1 = "xxx"; $2 = "xxx"; print }' \
	test/tmp/bench.1/stats.txt

awk -F'\t' '{ $1 = "xxx"; $2 = "xxx"; print }' \
	test/tmp/torrentdb/stats.txt > test/tmp/torrentdb/stats.txt.nodates

cmp test/tmp/torrentdb/stats.txt.nodates test/tmp/bench.1/stats.txt
cmp test/tmp/torrentdb/files.tsv test/tmp/bench.1/files.tsv
cmp test/tmp/torrentdb/torrents.tsv test/tmp/bench.1/torrents.tsv

echo '* all good'



echo '\n*** pass 2 - preparing db\n'

./bin/torrentdb -d test/tmp/torrentdb -t test/torsniff_dump.2



echo '\n*** pass 2 - testing the output\n'

cp -a test/bench.2 test/tmp

awk -i inplace -F'\t' '{ $1 = "xxx"; $2 = "xxx"; print }' \
	test/tmp/bench.2/stats.txt

awk -F'\t' '{ $1 = "xxx"; $2 = "xxx"; print }' \
	test/tmp/torrentdb/stats.txt > test/tmp/torrentdb/stats.txt.nodates

sed -i "s/YYYY-MM-DD/$(date +%Y-%m-%d)/g" test/tmp/bench.2/torrents.tsv
cut -d' ' -f 3- test/tmp/torrentdb/error.log > test/tmp/torrentdb/error.log.nodates

cmp test/tmp/torrentdb/stats.txt.nodates test/tmp/bench.2/stats.txt
cmp test/tmp/torrentdb/files.tsv test/tmp/bench.2/files.tsv
cmp test/tmp/torrentdb/torrents.tsv test/tmp/bench.2/torrents.tsv
cmp test/tmp/torrentdb/error.log.nodates test/tmp/bench.2/error.log

echo '* all good'
