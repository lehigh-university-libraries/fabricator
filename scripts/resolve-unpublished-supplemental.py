#!/usr/bin/env python3

import csv
import sys
from pathlib import Path


INPUT_DIR = Path("input_data")
PENDING = INPUT_DIR / "target.unpublished_supplemental.csv"
CREATE_CSV = INPUT_DIR / "target.csv"
ROLLBACK_CSV = INPUT_DIR / "rollback.csv"
ADD_MEDIA_CSV = INPUT_DIR / "target.add_media.csv"
ADD_MEDIA_FIELDS = ["node_id", "file", "media_use_tid", "published"]


def read_csv_rows(path):
    with path.open(newline="") as fh:
        return list(csv.DictReader(fh))


def create_id_to_node_id():
    if not CREATE_CSV.exists():
        return {}
    if not ROLLBACK_CSV.exists():
        raise RuntimeError("rollback.csv does not exist; cannot resolve unpublished supplemental media node IDs")

    create_rows = read_csv_rows(CREATE_CSV)
    create_ids = [row.get("id", "").strip() for row in create_rows]
    node_ids = []
    with ROLLBACK_CSV.open(newline="") as fh:
        for row in csv.reader(line for line in fh if not line.startswith("#")):
            if not row:
                continue
            node_id = row[0].strip()
            if node_id.isdigit():
                node_ids.append(node_id)

    if len(node_ids) < len(create_ids):
        raise RuntimeError(
            f"rollback.csv has {len(node_ids)} node IDs but target.csv has {len(create_ids)} create rows"
        )

    return {upload_id: node_ids[index] for index, upload_id in enumerate(create_ids) if upload_id}


def existing_add_media_rows():
    if not ADD_MEDIA_CSV.exists():
        return []
    rows = read_csv_rows(ADD_MEDIA_CSV)
    normalized = []
    for row in rows:
        normalized.append({field: row.get(field, "").strip() for field in ADD_MEDIA_FIELDS})
    return normalized


def write_add_media(rows):
    with ADD_MEDIA_CSV.open("w", newline="") as fh:
        writer = csv.DictWriter(fh, fieldnames=ADD_MEDIA_FIELDS)
        writer.writeheader()
        writer.writerows(rows)


def main():
    if not PENDING.exists():
        return 0

    id_to_node_id = create_id_to_node_id()
    add_media_rows = existing_add_media_rows()
    for row in read_csv_rows(PENDING):
        node_id = row.get("node_id", "").strip()
        if not node_id:
            upload_id = row.get("id", "").strip()
            node_id = id_to_node_id.get(upload_id, "")
        if not node_id:
            raise RuntimeError(f"could not resolve node_id for unpublished supplemental file {row.get('file', '')}")

        add_media_rows.append(
            {
                "node_id": node_id,
                "file": row.get("file", "").strip(),
                "media_use_tid": row.get("media_use_tid", "").strip() or "151326",
                "published": "0",
            }
        )

    write_add_media(add_media_rows)
    PENDING.unlink()
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:
        print(exc, file=sys.stderr)
        raise SystemExit(1)
