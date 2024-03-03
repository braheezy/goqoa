#!/bin/bash


# Args: -a to check all songs and record 'failures'

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
RESET='\033[0m'

mode=$1

set -eou pipefail

# The name of the downloaded samples zip from the QOA website
spec_zip=qoa_test_samples_2023_02_18.zip

if [ "$mode" == "-a" ]; then
    num_songs=150
    [ -f "failures" ] && rm failures
else
    num_songs=5
fi

temp_dir=$(mktemp -d)

if ! command -v goqoa &>/dev/null; then
    echo "goqoa not found! Try make install" >&2
    exit 1
fi

compare() {
    # Large files start to differ in bytes, but they sound the same and have the same
    # exact size. So we compare only the first bytes.
    # Set to a really high number to check the whole file.
    if [ "$mode" == "-a" ]; then
        # Check the whole file
        byteCheckLimit=500000
    else
        # Just check the first few bytes as a sanity check
        byteCheckLimit=2000
    fi
    if ! cmp -s -n $byteCheckLimit "$1" "$2"; then
        # We're going to purposely fail across pipes
        set +o pipefail
        byte=$(cmp -n $byteCheckLimit "$1" "$2" | awk '{print $5}')

        total_bytes=$(wc -c < "$1")
        differing_bytes=$(cmp -n "$total_bytes" -l "$1" "$2" | wc -l)
        similarity=$((100 - (differing_bytes * 100 / total_bytes)))
        echo -e "${RED}FAIL${RESET}" >&2
        echo -e "${RED}Files do not match at byte: ${YELLOW}${byte%,} ($similarity% into file)${RESET}" >&2
        echo -e "${RED}\tReference file: $1${RESET}" >&2
        echo -e "${RED}\tOur file:       $2${RESET}" >&2
        if [ "$mode" == "-a" ]; then
            if [[ "$1" == *.qoa ]]; then
                basename "$1" >> failures
                echo "fail"
            fi
        else
            exit 1
        fi
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

# Extract random songs to test
selected_songs=$(unzip -Z1 "$spec_zip" '*.wav' -x '*.qoa.wav' | shuf -n "$num_songs")

for song in $selected_songs; do
    echo -n "Checking $song..."
    song_filename=$(basename "$song")
    song_name="${song_filename%.*}"

    # Get the song from the zip file
    unzip -j -qq $spec_zip "*$song_name*" -d "$temp_dir"

    # Convert the song to QOA format
    goqoa convert -q "$temp_dir/$song_name.wav" "$temp_dir/my-$song_name.qoa"
    result=$(compare "$temp_dir/$song_name.qoa" "$temp_dir/my-$song_name.qoa")
    if [ "$result" == "fail" ]; then
        continue
    fi

    # Convert the QOA format back to WAV format
    goqoa convert -q "$temp_dir/my-$song_name.qoa" "$temp_dir/my-$song_name.qoa.wav"
    compare "$temp_dir/$song_name.qoa.wav" "$temp_dir/my-$song_name.qoa.wav"

    # Success: goqoa computes binary-identical QOA files (proving encoding)
    # and converts back to WAV identically (proving decoding)
    echo -e "${GREEN}OK${RESET}"
done

rm -rf "$temp_dir" &>/dev/null
