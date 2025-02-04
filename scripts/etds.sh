#!/usr/bin/env bash

set -eou pipefail

WORKING_DIR="$HOME/etds"
cd "$WORKING_DIR"

RUN=false
for ZIP in /mnt/islandora_staging/etd_uploads/*.zip; do
  FILE=$(basename "$ZIP");
  if [ ! -f "$WORKING_DIR/processed/$FILE" ]; then
    cp "$ZIP" "$WORKING_DIR/$FILE"
    RUN=true
    unzip "$FILE"
  fi
done

if [ "$RUN" = false ]; then
  echo "No new ZIPs"
  exit 0
fi

echo "Downloading latest go-islandora"
ARCH="go-islandora_Linux_x86_64.tar.gz"
TAG=$(gh release list --exclude-pre-releases --exclude-drafts --limit 1 --repo lehigh-university-libraries/go-islandora | awk '{print $3}')
gh release download "$TAG" --repo lehigh-university-libraries/go-islandora --pattern "$ARCH"
tar -zxvf "$ARCH"
rm "$ARCH"

DATE=$(date +"%Y-%m-%d")
CSV="etds-$DATE.csv"
echo "Transforming ZIP files to CSV"
./go-islandora --source "$(pwd)" --target "$CSV"

echo "Uploading CSV to Google Sheets"
GSHEET=$(./go-islandora transform csv --source "$CSV" --folder "$FOLDER_ID")

echo "Starting ingest"
