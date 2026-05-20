#!/bin/bash

# Script for generating dummy albums with random tracks and metadata for testing purposes.
# Example usage: ./generate_album.sh /path/to/destination 5

# Argument 1: Destination directory (defaults to current dir)
DEST_DIR="${1:-.}"
# Argument 2: Number of albums to generate (defaults to 3)
NUM_ALBUMS="${2:-3}"

mkdir -p "$DEST_DIR"

echo "Generating $NUM_ALBUMS albums in: $DEST_DIR"

for a in $(seq 1 "$NUM_ALBUMS"); do
  NUM_TRACKS=$((RANDOM % 15 + 6))
  
  ALBUM_NAME="The Music Vol. $a"
  ALBUM_ARTIST="The Placeholders"
  ALBUM_YEAR=$((RANDOM % 20 + 2005)) 
  ALBUM_COLOR=$(printf "%06x\n" $((RANDOM * RANDOM % 16777216)))
  
  ALBUM_DIR="$DEST_DIR/Album_$a"
  mkdir -p "$ALBUM_DIR"
  
  echo "-> Creating '$ALBUM_NAME' ($NUM_TRACKS tracks) in $ALBUM_DIR"

  ffmpeg -loglevel error -f lavfi -i color=c=0x${ALBUM_COLOR}:s=500x500 -frames:v 1 -y "$ALBUM_DIR/cover.jpg"

  for t in $(seq 1 "$NUM_TRACKS"); do
    TRACK_NUM=$(printf "%02d" "$t")
    TRACK_TITLE="Dummy Track $t"
    
    TRACK_DURATION=$((120 + RANDOM % 60))          
    TRACK_DURATION_MS=$((TRACK_DURATION * 1000))   
    
    # NOTE: -t is now applied directly to the audio input to guarantee length
    ffmpeg -loglevel error -f lavfi -t "$TRACK_DURATION" -i anullsrc=r=44100:cl=stereo \
    -i "$ALBUM_DIR/cover.jpg" \
    -map 0:a -map 1:v \
    -metadata title="$TRACK_TITLE" \
    -metadata artist="$ALBUM_ARTIST" \
    -metadata album="$ALBUM_NAME" \
    -metadata album_artist="$ALBUM_ARTIST" \
    -metadata track="$t/$NUM_TRACKS" \
    -metadata date="$ALBUM_YEAR" \
    -metadata TLEN="$TRACK_DURATION_MS" \
    -c:a libmp3lame -b:a 128k \
    -c:v copy -id3v2_version 3 -disposition:v attached_pic \
    -y "$ALBUM_DIR/track_${TRACK_NUM}.mp3"
  done
done

echo "Done! Generated $NUM_ALBUMS albums."