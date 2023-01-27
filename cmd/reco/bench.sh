#!/bin/bash

set -e

dir="$( cd "$(dirname "$0")"/../.. && pwd)"
file=${1:-$dir/README.md}

cat <<EOF  | while read -r ext level; do
s2     1
s2     2
s2     3
s2     4
gz    -3
gz    -2
gz    -1
gz     0
gz     1
gz     2
gz     3
gz     4
gz     5
gz     6
gz     7
gz     9
gz     9
zst    1
zst    2
zst    3
zst    4
br     0
br     1
br     2
br     3
br     4
br     5
br     6
br     7
br     8
br     9
br    10
br    11
EOF
    echo -e "$ext\t$level\t$( (
        set -x
        go run ./cmd/reco -loops 0 -level "$level" "$file" "$file.$ext"
    ) | grep Min)\t$(du -b "$file.$ext")"
done
