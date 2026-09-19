# Publication specialist contract

Inputs: request.json and all explicitly selected regular inputs/ files. Read inputs/source.json for authoritative facts; additional story/design files are selected by the controller. No prior-run discovery.

Output output/spec.json has exactly schema,role,request_sha256,inputs,content. schema is bench.publication-spec/v1. role is executive-writer. Bind SHA256 of exact request bytes; inputs is a path-sorted list of {path,sha256} for every regular selected input recursively, paths beginning inputs/. No symlinks, duplicate JSON keys or nonfinite values.

content has title, subtitle, executive_summary (250-2200 characters), sections (5-9). Each section is exactly {heading,paragraphs,fact_ids,message_ids,visual_id}. Use 2-6 sustained paragraphs per section. Across summary and sections write 1100-3000 words with no filler. visual_id is null or an ID in inputs/design.json. Cover every required story message across sections. tools/render makes editable DOCX, PDF and Markdown with relevant figures and page previews. Source IDs are for provenance; speak naturally to the reader.

Compute hashes only after final input selection. Run this definition’s absolute bin/check from the workspace. Production roles first invoke this definition’s tools/render; it reads declarative spec only. Source helpers and dependency runtime are host-controlled. A passed file check establishes structure and binding, not quality. An independent publication review covers every format and actual rendered previews before release.
