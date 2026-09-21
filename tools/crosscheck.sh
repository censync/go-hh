#!/bin/sh
# Differential test of go-hh against hh-cpp. Two batches go through hh_cli of hh-cpp and through
# cmd/hh-cli here: pseudo-random cases, valid and invalid, and the hand-made cases of
# tools/edge-cases.txt, which pin down the batch format itself. Every printed value and every
# written file (PNG, BMP, JPEG, raw pixels) must be identical.
#
# usage: tools/crosscheck.sh <path to hh_cli> [case count] [seed]
set -eu

here=$(cd "$(dirname "$0")/.." && pwd)
hh_cli=${1:?usage: tools/crosscheck.sh <path to hh_cli> [case count] [seed]}
count=${2:-400}
seed=${3:-1}
work=$here/build/crosscheck
rm -rf "$work"
mkdir -p "$work"

go_cli=$work/hh-cli
(cd "$here" && go build -o "$go_cli" ./cmd/hh-cli)

"$go_cli" --generate "$count" "$seed" > "$work/generated.txt"

# compare <name> <case file>: runs one batch through both tools.
compare() {
    "$hh_cli" --batch "$2" "$work/$1-cpp" > "$work/$1-cpp.txt"
    "$go_cli" --batch "$2" "$work/$1-go" > "$work/$1-go.txt"
    if ! diff "$work/$1-cpp.txt" "$work/$1-go.txt" > "$work/$1-values.diff"; then
        echo "crosscheck: the printed values of the $1 cases differ, see $work/$1-values.diff" >&2
        head -n 20 "$work/$1-values.diff" >&2
        exit 1
    fi
    if ! diff -r "$work/$1-cpp" "$work/$1-go" > "$work/$1-files.diff"; then
        echo "crosscheck: the written files of the $1 cases differ, see $work/$1-files.diff" >&2
        head -n 20 "$work/$1-files.diff" >&2
        exit 1
    fi
}

compare generated "$work/generated.txt"
compare edge "$here/tools/edge-cases.txt"

edge=$(wc -l < "$work/edge-cpp.txt" | tr -d ' ')
ok=$(cat "$work/generated-cpp.txt" "$work/edge-cpp.txt" | grep -c "	ok	" || true)
files=$(find "$work/generated-cpp" "$work/edge-cpp" -type f | wc -l | tr -d ' ')
echo "crosscheck: $count generated cases and $edge edge cases agree" \
    "($ok rendered, $files files compared byte for byte)"
