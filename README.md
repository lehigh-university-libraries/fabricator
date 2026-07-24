# fabricator

Transform a Google Sheet URL into a fully executed [Islandora Workbench](https://mjordan.github.io/islandora_workbench_docs/) task.

## Overview

Content creators can work in Google Sheets to prepare a spreadsheet for bulk ingest. The Google Sheet can be structured in a manor that makes populating the content much more user friendly than the format a workbench CSV may require. e.g. some of Lehigh's fields need populated by workbench using JSON. Instead of asking a person to hand craft JSON, we can just create multiple columns that each contain a value that populates a given JSON field, and we can automate aggregating those columns and representing them in the correct JSON format

[A Google Appscript](./google/appsscript) is embeded in the sheet to allow easily checking the data in the spreadsheet is valid

When the spreadsheet is ready to have the content added to the repository, the metadata in the spreadsheet can be ingested into Islandora/Drupal via Islandora Workbench by [supplying the sheet URL in the GitHub Action](../.././actions/workflows/run.yml). Write access on this repo is required to execute the workflow

The GitHub Action is executed on a self-hosted runner within Lehigh's infrastructure. This allows uploading files directly from the same file server Lehigh staff use. This also allows ensuring the files referenced in the Google Sheet exist before executing the workbench job.

```mermaid
sequenceDiagram
    actor Alice
    participant Google Sheets
    participant Fabricator
    participant GitHub
    participant Slack
    Alice->>Google Sheets: Edit 1
    Alice->>Google Sheets: Edit 2
    Alice->>Google Sheets: Edit ...
    Alice->>Google Sheets: Edit N
    Alice->>Google Sheets: Click check my work
    Google Sheets->>Fabricator: Check this CSV
    Fabricator->>Alice: Looks good 🚀
    Alice->>GitHub: Run workbench workflow
    GitHub->>Self-hosted Runner: Run workbench workflow
    Self-hosted Runner->>Slack: Workbench job started
    Slack->>Alice: Message notification
    Self-hosted Runner->>Islandora Workbench: python3 workbench
    Islandora Workbench->>Drupal: entity CUD
    Islandora Workbench->>GitHub: logs streamed to GitHub Action UI
    Alice->>GitHub: Clicks Slack link to view GitHub Action logs while job runs
    Self-hosted Runner->>Slack: ✅ Workbench job succeeded!
```

## Workflow scenarios

Fabricator chooses the Workbench task from the transformed CSV headers:

- `target.csv` means a create task.
- `target.update.csv` means a node metadata update task.
- `target.add_media.csv` means an add-media task for existing nodes.
- `target.unpublished_supplemental.csv` is an internal follow-up file. It is resolved into `target.add_media.csv` after node creation so unpublished supplemental media can be attached to newly created nodes.

<details>
<summary>Create task, with optional spin-offs</summary>

This is the most common path. Rows without `Node ID` become new Islandora nodes. Regular files and regular supplemental files are handled by the create task. Unpublished supplemental files require a post-create add-media pass because their parent node IDs do not exist until Workbench finishes creating nodes.

```mermaid
flowchart TD
    A[Google Sheet CSV] --> B[Fabricator check]
    B --> C[Fabricator transform]
    C --> D[target.csv]
    C --> E{Unpublished Supplemental Files present?}
    C --> F{Contributor values include extra metadata?}

    F -->|yes| G[Resolve or create contributor terms]
    G --> D
    F -->|no| D

    D --> H[Workbench create.yml]
    H --> I[Create nodes]
    H --> J[Attach File Path media]
    H --> K[Attach Supplemental File media via additional_files]
    H --> L[Write rollback.csv with created node IDs]

    E -->|yes| M[target.unpublished_supplemental.csv]
    L --> N[Fabricator resolve-unpublished-supplemental]
    M --> N
    N --> O[target.add_media.csv with published=0]
    O --> P[Workbench add_media.yml]
    P --> Q[Attach unpublished supplemental media]

    E -->|no| R[No unpublished supplemental follow-up]
```

</details>

<details>
<summary>Node metadata update task</summary>

Rows with `Node ID` and metadata fields become an update task. Template-only create columns such as `Upload ID`, `Page/Item Parent ID`, and `File Path` are dropped so the CSV is treated as metadata update input, not create or add-media input.

```mermaid
flowchart TD
    A[Google Sheet CSV with Node ID] --> B[Fabricator check]
    B --> C[Fabricator transform]
    C --> D[Drop create-only columns]
    D --> E[target.update.csv]
    E --> F[Workbench update.yml]
    F --> G[Update node metadata]
```

</details>

<details>
<summary>Add media task for existing nodes</summary>

Rows with `Node ID` and `File Path`, without additional metadata fields, become an add-media task. If the same sheet also contains `Unpublished Supplemental Files`, Fabricator merges those rows into the add-media CSV and writes explicit published values so regular media remains published and unpublished supplemental media remains unpublished.

```mermaid
flowchart TD
    A[Google Sheet CSV with Node ID and File Path] --> B[Fabricator check]
    B --> C[Fabricator transform]
    C --> D[target.add_media.csv]
    C --> E{Unpublished Supplemental Files present?}

    D --> F[Regular media rows]
    F --> G[published=1 when merged]

    E -->|yes| H[target.unpublished_supplemental.csv]
    H --> I[Fabricator resolve-unpublished-supplemental]
    G --> I
    I --> J[target.add_media.csv]
    J --> K[Workbench add_media.yml]
    K --> L[Attach regular media as published]
    K --> M[Attach unpublished supplemental media as published=0]

    E -->|no| N[Workbench add_media.yml]
    D --> N
    N --> O[Attach regular media]
```

</details>

<details>
<summary>Contributor term resolution with extra metadata</summary>

Contributor cells can include extra metadata such as email, ORCiD, institution, and status. Fabricator resolves those contributors before writing the Workbench CSV value. If a person term does not exist, Fabricator creates it. If a unique identifier matches an existing person with a different name, Fabricator creates a child term so lineage is preserved.

```mermaid
flowchart TD
    A[Contributor cell JSON] --> B[Fabricator transform]
    B --> C{Vocabulary is person?}
    C -->|no| D[Pass through existing contributor value]
    C -->|yes| E{Email or ORCiD present?}
    E -->|yes| F[Lookup person by unique metadata]
    E -->|no| G[Lookup person by name and institution]
    F --> H{Matching term found?}
    G --> H
    H -->|yes, same name| I[Use existing term ID]
    H -->|yes, different name with unique match| J[Create child person term]
    H -->|no| K[Create person term]
    I --> L[Write field_linked_agent value]
    J --> L
    K --> L
```

</details>

## Technical details

This is an http service with two routes:

- `/workbench/check`
  - check if a google sheet content is well formed
- `/workbench/transform`
  - transform a google sheet CSV export into a workbench CSV

### Start the server

```
git clone https://github.com/lehigh-university-libraries/fabricator
docker build -t fabricator:main
docker run --rm -d -p 8080:8080 fabricator:main
```

### Ensure a google sheet CSV has no bad data

The `/workbench/check` route returns a JSON map keyed by the Google Sheet column/row of a cell and the error that cell contains. If the map is empty, there are no errors.

The route requires the CSV to be uploaded as JSON. This was done since the Google Sheets Appscript does not have a convenient SDK to convert a Google Sheet into a CSV. Instead, [the sheet is parsed cell by cell and stored as a JSON map](https://github.com/lehigh-university-libraries/fabricator/blob/86e77d8124dcbb522ca951ed3a1319e0193db73e/google/appsscript/check.gs#L18-L24). You can [see in the tests how the JSON is structured](https://github.com/lehigh-university-libraries/fabricator/blob/86e77d8124dcbb522ca951ed3a1319e0193db73e/internal/handlers/check_test.go#L81-L84).

There's also [an example script](./scripts/download.sh) on how to download a Google Sheet into the JSON format and also CSV format.

#### Example: no errors

```
$ curl -s \
  -H "X-Secret: $SHARED_SECRET" \
  -XPOST \
  --upload-file csv.json \
  http://localhost:8080/workbench/check
```
```
{}
```

#### Example: Row 12, Column A has a required field that is blank

```
$ curl -s \
  -H "X-Secret: $SHARED_SECRET" \
  -XPOST \
  --upload-file csv.json \
  http://localhost:8080/workbench/check
```
```
{"A12": "Missing value"}
```

### Get a workbench CSV from a google sheet CSV

The `/workbench/transform` route transforms a Google Sheet CSV into a Workbench CSV. The route returns a ZIP of CSVs. There are two possible flavors of CSVs that can be returned:

- target.csv - used to run [a workbench create task](./workbench-configs/create.yml)
  - this is the most common pattern used at Lehigh. This creates metadata and media/files for new content being added to the repository
- target.update.csv - used to run [a workbench updaye task](./workbench-configs/update.yml)
  - this is returned when the Google Sheet contains node IDs in the sheet, signifying the job should be updating metadata for existing nodes
- target.unpublished_supplemental.csv - used internally when `Unpublished Supplemental Files` is present
  - after the create task writes node IDs, this is resolved into `target.add_media.csv` and run with [the add_media task](./workbench-configs/add_media.yml) so those supplemental media are created unpublished

```
$ curl -s \
  -H "X-Secret: $SHARED_SECRET" \
  -XPOST \
  -o target.zip \
  --upload-file source.csv \
  http://localhost:8080/workbench/transform
$ unzip target.zip
```

## Adding a new MARC relator

1. Update this repo's relator list in
```
google/appsscript/contributor-form.html
internal/handlers/check.go
```
2. Update the Google Apps Script in [the metadata template spreadsheet](https://docs.google.com/spreadsheets/d/1iB7GsnfvhQO_c6TzJb7qwCnItqju0PMC8mNWepYqsnU/edit?gid=0#gid=0)
3. Update "Available Relations" field at <https://preserve.lehigh.edu/admin/structure/types/manage/islandora_object/fields/node.islandora_object.field_linked_agent>

## Adding new columns to the ingest template

If the ingest template needs a new column added, these are the code changes that are needed

- Add the column to [the ingest template](https://docs.google.com/spreadsheets/d/1iB7GsnfvhQO_c6TzJb7qwCnItqju0PMC8mNWepYqsnU/edit#gid=0), making row one the human-friendly label
- Make the necessary changes to [go-islandora](https://github.com/lehigh-university-libraries/go-islandora)
  - Add the column label and machine name to [the sheets slice in go-islandora](https://github.com/lehigh-university-libraries/go-islandora/blob/965bd728379bf2a9aa0ddb1fb46ec05fda636d87/cmd/sheetsStructs.go#L61)
  - generate the openapi schema and structs `go build && ./go-islandora generate sheets-structs --output=workbench.yaml`
- Make the necessary changes in this repo
  - Update go.mod `go get -u github.com/lehigh-university-libraries/go-islandora@main`
  - Add any necessary [checks](./internal/handlers/check.go) and [tests](./internal/handlers/check_test.go)
- Deploy the new image to the staging server
```
isle-stage
cd /opt/islandora/d10_lehigh_agile
sudo docker compose --profile prod pull
sudo systemctl restart islandora
```

TODO: This should eventually be able to be automatted, and the ingest template is simply generated by this repo ([Issue #23](https://github.com/lehigh-university-libraries/fabricator/issues/23)).
