#!/bin/ksh
set -eu

# to prepare benchamrk hash list:
# $ find . -name '*.torrent' -exec transmission-show {} \; | grep '  Hash: ' | # cut -d' ' -f4 > hash_list.txt
# $ sort -u -o hash_list.txt hash_list.txt

# to compare to torrentdb output:
# $ cat error.log | cut -d' ' -f3 | sort -u | awk 'length($0)>39' > _error.hash
# $ grep '^hash: ' files.tsv | cut -d' ' -f2 > _files.hash
# $ cat torrents.tsv | cut -f1 | sort -u > _torrents.hash

# total line count should be the same (715082): 
# $ wc -l hash_list.txt
# $ wc -l _error.hash _torrents.hash
# $ wc -l _error.hash _files.hash
# $ wc -l _error.hash torrents.tsv



# benchmark times:
# *** pass 1 - preparing db
# time: 189

# *** pass 2 - preparing db
# time: 238

# *** pass 3 - preparing db
# time: 187



LINE_COUNT_TORRENTS=711592
HASH_COUNT_ERROR=3490

rm -r test/tmp
mkdir -p test/tmp/torrentdb



echo '\n*** pass 1 - preparing db\n'

find ../torrent_utils_test/torsniff_dump.1 -type f -name '*.torrent' \
	-exec touch -m -d '2002-05-14' {} \+

SECONDS=0
./bin/torrentdb -d test/tmp/torrentdb -t ../torrent_utils_test/torsniff_dump.1
echo "\ntime: $SECONDS"



echo '\n*** pass 2 - preparing db\n'

find ../torrent_utils_test/torsniff_dump.2 -type f -name '*.torrent' \
	-exec touch {} \+

SECONDS=0
./bin/torrentdb -d test/tmp/torrentdb -t ../torrent_utils_test/torsniff_dump.2
echo "\ntime: $SECONDS"



echo '\n*** pass 3 - preparing db\n'

find ../torrent_utils_test/torsniff_dump.3 -type f -name '*.torrent' \
	-exec touch {} \+

SECONDS=0
./bin/torrentdb -d test/tmp/torrentdb -t ../torrent_utils_test/torsniff_dump.3
echo "\ntime: $SECONDS"



echo '\n*** making a copy of benchmark files'

cp -a ../torrent_utils_test/bench test/tmp



echo '*** preparing benchmarks'

awk -i inplace -F'\t' '{ $1 = "xxx"; $2 = "xxx"; print }' \
	test/tmp/bench/stats.txt

awk -F'\t' '{ $1 = "xxx"; $2 = "xxx"; print }' \
	test/tmp/torrentdb/stats.txt > test/tmp/torrentdb/stats.txt.nodates

sed -i "s/YYYY-MM-DD/$(date +%Y-%m-%d)/g" test/tmp/bench/torrents.tsv
cut -d' ' -f 3- test/tmp/torrentdb/error.log > test/tmp/torrentdb/error.log.nodates

cat test/tmp/torrentdb/error.log |
	cut -d' ' -f3 |
	sort -u |
	awk 'length($0)>39' > test/tmp/torrentdb/_error.hash



echo '*** testing the output'

cmp test/tmp/torrentdb/stats.txt.nodates test/tmp/bench/stats.txt
cmp test/tmp/torrentdb/files.tsv test/tmp/bench/files.tsv
cmp test/tmp/torrentdb/torrents.tsv test/tmp/bench/torrents.tsv
cmp test/tmp/torrentdb/error.log.nodates test/tmp/bench/error.log

[ $(wc -l test/tmp/torrentdb/torrents.tsv | cut -d' ' -f1) -eq ${LINE_COUNT_TORRENTS} ] || exit 1
[ $(wc -l test/tmp/torrentdb/_error.hash  | cut -d' ' -f1) -eq ${HASH_COUNT_ERROR} ] || exit 1

echo '\n* all good'
