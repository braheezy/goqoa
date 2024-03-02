#!/bin/bash

set -eou pipefail

spec_zip=qoa_test_samples_2023_02_18.zip

num_songs=10

temp_dir=$(mktemp -d)

if ! command -v goqoa &>/dev/null; then
    echo "goqoa not found! Try make install"
fi

size_compare() {
    size1=$(stat -c %s "$1")
    size2=$(stat -c %s "$2")

    if [ "$size1" != "$size2" ]; then
        echo "Sizes do not match!"
        echo "$1: $size1"
        echo "$2: $size2"
        exit 1
    fi
}

if [ ! -f $spec_zip ]; then
    echo "Downloading $spec_zip..."
    if command -V http &>/dev/null; then
        http -d https://qoaformat.org/samples/qoa_test_samples_2023_02_18.zip
    else
        curl -s -O https://qoaformat.org/samples/qoa_test_samples_2023_02_18.zip
    fi
fi

echo "Processing $spec_zip..."
# Extract random songs
selected_songs=$(unzip -Z1 "$spec_zip" '*.wav' -x '*.qoa.wav' | shuf -n "$num_songs")

for song in $selected_songs; do
    echo -n "Checking $song..."
    song_filename=$(basename "$song")
    song_name="${song_filename%.*}"

    echo "Extracting $song_name..."
    unzip -j -qq $spec_zip "*$song_name*" -d "$temp_dir"

    echo "Converting $song_name.wav to qoa..."
    ls "$temp_dir/$song_name.wav"
    goqoa -q convert "$temp_dir/$song_name.wav" "$temp_dir/my-$song_name.qoa"
    size_compare "$temp_dir/$song_name.qoa" "$temp_dir/my-$song_name.qoa"

    echo "Converting $song_name back to wav..."
    goqoa -q convert "$temp_dir/my-$song_name.qoa" "$temp_dir/my-$song_name.qoa.wav"
    size_compare "$temp_dir/$song_name.qoa.wav" "$temp_dir/my-$song_name.qoa.wav"

    echo "OK"
done

rm -rf "$temp_dir" &>/dev/null
