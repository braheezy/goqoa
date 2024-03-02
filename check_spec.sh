#!/bin/bash

set -eou pipefail

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
RESET='\033[0m'

spec_zip=qoa_test_samples_2023_02_18.zip

num_songs=10

temp_dir=$(mktemp -d)

if ! command -v goqoa &>/dev/null; then
    echo "goqoa not found! Try make install" >&2
    exit 1
fi

compare() {
    # Large files start to differ in bytes, but they sound the same and have the same
    # exact size. So we compare only the first bytes.
    byteCheckLimit=9000
    if ! cmp -s -n $byteCheckLimit "$1" "$2"; then
        set +o pipefail
        byte=$(cmp -n $byteCheckLimit "$1" "$2" | awk '{print $5}')
        # echo "$byte"
        echo -e "${RED}FAIL${RESET}"
        echo -e "${RED}Files do not match at byte: ${YELLOW}$byte${RESET}" >&2
        echo -e "${RED}\t$1${RESET}" >&2
        echo -e "${RED}\t$2${RESET}" >&2
        exit 1
    fi
}

if [ ! -f $spec_zip ]; then
    echo "Downloading $spec_zip..."
    if command -V http &>/dev/null; then
        http --download --body https://qoaformat.org/samples/qoa_test_samples_2023_02_18.zip
    else
        curl -s -O https://qoaformat.org/samples/qoa_test_samples_2023_02_18.zip
    fi
fi

# Extract random songs
selected_songs=$(unzip -Z1 "$spec_zip" '*.wav' -x '*.qoa.wav' | shuf -n "$num_songs")

for song in $selected_songs; do
    echo -n "Checking $song..."
    song_filename=$(basename "$song")
    song_name="${song_filename%.*}"

    unzip -j -qq $spec_zip "*$song_name*" -d "$temp_dir"

    goqoa convert -q "$temp_dir/$song_name.wav" "$temp_dir/my-$song_name.qoa"
    compare "$temp_dir/$song_name.qoa" "$temp_dir/my-$song_name.qoa"

    goqoa convert -q "$temp_dir/my-$song_name.qoa" "$temp_dir/my-$song_name.qoa.wav"
    compare "$temp_dir/$song_name.qoa.wav" "$temp_dir/my-$song_name.qoa.wav"

    echo -e "${GREEN}OK${RESET}"
done

rm -rf "$temp_dir" &>/dev/null
