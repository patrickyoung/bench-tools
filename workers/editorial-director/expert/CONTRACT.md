# Publication specialist contract

Inputs: request.json and all explicitly selected regular inputs/ files. Read inputs/source.json for authoritative facts; additional story/design files are selected by the controller. No prior-run discovery.

Output output/spec.json has exactly schema,role,request_sha256,inputs,content. schema is bench.publication-spec/v1. role is editorial-director. Bind SHA256 of exact request bytes; inputs is a path-sorted list of {path,sha256} for every regular selected input recursively, paths beginning inputs/. No symlinks, duplicate JSON keys or nonfinite values.

content has title, subtitle, audience, thesis (strings); messages (3-7 records: id,text,qualification,fact_ids); required_message_ids (2-7 IDs); terminology (string list); report_arc (5-9 headings); deck_arc (7-12 slide topics); design {direction,font,ink,accent,secondary,paper}. Colors are #RRGGBB. Font: Arial, Helvetica or Liberation Sans. Every message cites supplied source.facts IDs and states its material qualification.

Compute hashes only after final input selection. Run this definition’s absolute bin/check from the workspace. Production roles first invoke this definition’s tools/render; it reads declarative spec only. Source helpers and dependency runtime are host-controlled. A passed file check establishes structure and binding, not quality. An independent publication review covers every format and actual rendered previews before release.
