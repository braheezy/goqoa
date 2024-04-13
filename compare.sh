#!/bin/bash

set -ou pipefail

usage() {
    echo "Usage: $0 <file|directory|archive>"
    exit 1
}

# Function to handle regular files
process_file() {
    local file=$1
    echo "Processing file: $file"
    song_filename=$(basename "$file")
    song_name="${song_filename%.*}"

    result=$(qoaconv "$file" "/data/output_qoa/$song_name.qoa")
    if [ $? -ne 0 ]; then
        echo "qoaconv,$file,error: see raw_output.txt" >> /data/raw_output.txt
    else
        echo "$result" >> /data/raw_output.txt
    fi

    result=$(goqoa convert -v "$file" "/data/output_qoa/$song_name.qoa")
    if [ $? -ne 0 ]; then
        echo "goqoa,$file,error: see raw_output.txt" >> /data/raw_output.txt
    else
        echo "$result" >> /data/raw_output.txt
    fi
}

# Function to handle directories
process_directory() {
    local dir=$1
    echo "Processing directory: $dir"
    for file in "$dir"/*.wav; do
        process_file "$file"
    done
}

# Function to handle archive files
process_archive() {
    local archive=$1
    echo "Processing archive: $archive"
    # The name of the downloaded samples zip from the QOA website
    spec_zip=/data/qoa_test_samples_2023_02_18.zip

    if [ ! -f "$spec_zip" ]; then
        echo "Can't find $spec_zip"
        exit 1
    fi

    num_songs=10
    # Extract random songs to test
    selected_songs=$(unzip -Z1 "$spec_zip" '*.wav' -x '*.qoa.wav' | shuf -n "$num_songs")

    for song in $selected_songs; do
        song_filename=$(basename "$song")
        song_name="${song_filename%.*}"
        # Get the song from the zip file
        unzip -j -qq $spec_zip "*$song_name*" -d "$temp_dir"

        qoaconv "$temp_dir/$song_name.wav" "/data/output_qoa/$song_name.qoa" &>> /data/raw_output.txt
        goqoa convert -v "$temp_dir/$song_name.wav" "/data/output_goqoa/$song_name.go.qoa" &>> /data/raw_output.txt
    done
    rm -rf "$temp_dir"

    grep --color=always -i psnr /data/raw_output.txt
}

if [ -z "${1+x}" ]; then
    usage
fi
file_path=$1

if [ ! -e "$file_path" ]; then
    echo "The specified file or directory does not exist."
    exit 1
fi


mkdir -p /data/output_qoa
mkdir -p /data/output_goqoa
rm -f /data/raw_output.txt &>/dev/null
temp_dir=$(mktemp -d)

if [ -f "$file_path" ]; then
    # Check if it's an archive
    if file "$file_path" | grep -qE 'Zip archive data'; then
        process_archive "$file_path"
    else
        process_file "$file_path"
    fi
elif [ -d "$file_path" ]; then
    process_directory "$file_path"
else
    echo "Unsupported file type."
    exit 1
fi

# Function to extract and format data as CSV
extract_and_format_csv() {
    local raw_output="/data/raw_output.txt"
    local csv="encoder,file,psnr,channels,sample rate,duration,size,bitrate\n"

    local file=""
    local channels=""
    local sample_rate=""
    local duration=""
    local size=""
    local bitrate=""
    local psnr=""

    # Assuming that each entry follows the format shown in your example
    while IFS= read -r line; do
        if [[ "$line"  =~ channels: ]]; then
            channels=$(echo "$line" | awk -F'channels: ' '{print $2}' | awk '{print $1}' | sed 's/,*$//g')
            sample_rate=$(echo "$line" | awk -F'samplerate: ' '{print $2}' | awk '{print $1, $2}'| sed 's/,*$//g')
            duration=$(echo "$line" | awk -F'duration: ' '{print $2}')
        fi
        if [[ "$line" =~ ^/data/output_.* ]]; then
            file=$(echo "$line" | awk '{print $1}' | sed 's/:$//')
            file=$(basename "$file")
            size=$(echo "$line" | awk -F'size: ' '{print $2}' | awk '{print $1, $2}')
            bitrate=$(echo "$line" | awk -F'size: ' '{print $2}' | awk '{print $6, $7}' | sed 's/,*$//g')
            psnr=$(echo "$line" | awk -F'psnr: ' '{print $2}' | awk '{print $1, $2}')
            csv+="qoaconv,$file,$psnr,$channels,$sample_rate,$duration,$size,$bitrate\n"
        fi
        if [[ "$line"  =~ channels= ]]; then
            file=$(echo "$line" | awk '{print $2}')
            file=$(basename "$file")
            channels=$(echo "$line" | awk -F'channels=' '{print $2}' | awk '{print $1}')
            sample_rate=$(echo "$line" | awk -F'samplerate(hz)=' '{print $1}' | awk '{print $4}'  | cut -d'=' -f2)
            sample_rate="${sample_rate} hz"
            duration=$(echo "$line" | awk -F'duration=' '{print $2}' | sed 's/"//g')
        fi
        if [[ "$line"  =~ bitrate= ]]; then
            size=$(echo "$line" | awk -F'size=' '{print $2}' | awk '{print $1, $2}' | sed 's/"//g')
            bitrate=$(echo "$line" | awk -F'bitrate=' '{print $2}' |  awk '{print $1, $2}' | sed 's/"//g')
            psnr=$(echo "$line" | awk -F'psnr=' '{print $2}' | awk '{print $1}')
            psnr="${psnr} db"
            csv+="goqoa,$file,$psnr,$channels,$sample_rate,$duration,$size,$bitrate\n"
        fi
        if [[ "$line"  =~ error ]]; then
            csv+="${line},---,---,---,---,---\n"
        fi
    done < "$raw_output"

    echo "${csv}"
}

# Use gum to print the CSV as a table
csv_data=$(extract_and_format_csv)
if [ -z "${csv_data+x}" ]; then
    echo "No data found..."
    echo "Check raw_output.txt"
    exit 1
fi
echo -e "$csv_data" | gum table --print \
    --border.foreground "#DDB6F2" \
    --cell.foreground="#FAE3B0" \
    --header.foreground="#96CDFB"
