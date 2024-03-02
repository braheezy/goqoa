#!/bin/bash

set -eou pipefail

spec_zip=qoa_test_samples_2023_02_18.zip

num_songs=10

temp_dir=$(mktemp -d)

if ! command -v goqoa &>/dev/null; then
    echo "goqoa not found! Try make install" >&2
    exit 1
fi

compare() {
    # Large files start to differ in bytes, but they sound the same and have the same
    # exact size. So we compare only the first 20000 bytes.
    if ! cmp -n 20000 "$1" "$2"; then
        echo "Files do not match!" >&2
        echo "$1" >&2
        echo "$2" >&2
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

    echo "OK"
done

rm -rf "$temp_dir" &>/dev/null
