# TGN fixtures

`places.json` contains reduced Getty Thesaurus of Geographic Names responses
captured on 2026-09-21 from `https://vocab.getty.edu/tgn/{id}.json`, starting with
Bethlehem (7013416), Coplay (2087483), and Luxembourg (7003514) and following each
first parent to World.

Record IDs, labels, place-type IDs, parent order, and coordinate values are
preserved. Unused descriptive metadata is omitted. Tests rewrite TGN record URLs
to the local server; AAT classification URLs remain unchanged.

Source: [Getty Vocabularies Linked Open Data](https://www.getty.edu/research/tools/vocabularies/lod/).
Getty makes TGN data available under the
[Open Data Commons Attribution License](https://opendatacommons.org/licenses/by/1-0/).
