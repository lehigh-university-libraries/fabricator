# fabricator

Prepare a CSV to load via Islandora Workbench

This is a convenience utility to allow a more user friendly spreadsheet to then be converted to the format Workbench expects. Can be thought of as middleware between normal spreadsheet curation and the format workbench expects.

## Overview

```mermaid
sequenceDiagram
    actor Alice
    Alice->>Google Sheets: Edit 1
    Alice->>Google Sheets: Edit 2
    Alice->>Google Sheets: Edit ...
    Alice->>Google Sheets: Edit N
    Google Sheets->>Alice: <br>Download CSV
    Alice->>Fabricator: template.csv
    Fabricator->>Fabricator: processing/validating
    Fabricator->>Alice: workbench.csv
    Alice->>Islandora Workbench: workbench.csv
    Islandora Workbench->>Drupal: entity CUD
```

## Getting started

```
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
go generate ./api
```


## TODO
- [ ] HTTP service to allow a Google Sheets Apps script to validate a spreadsheet
- [ ] Validator service
- [ ] CSV transform service
