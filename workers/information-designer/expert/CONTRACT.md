# Publication specialist contract

Inputs: request.json and all explicitly selected regular inputs/ files. Read inputs/source.json for authoritative facts; additional story/design files are selected by the controller. No prior-run discovery.

Output output/spec.json has exactly schema,role,request_sha256,inputs,content. schema is bench.publication-spec/v1. role is information-designer. Bind SHA256 of exact request bytes; inputs is a path-sorted list of {path,sha256} for every regular selected input recursively, paths beginning inputs/. No symlinks, duplicate JSON keys or nonfinite values.

content has title, workbook_intro, visuals (3-6). Each visual is exactly {id,kind,title,subtitle,caption,alt,fact_ids,message_ids,steps}. ID is lowercase kebab text; kinds: score_bounds, score_heatmap, coverage, effects, decision_path, sensitivity. Require score_bounds and decision_path. Effects requires actual inferential comparisons. Titles/captions/alt preserve meaning. All quantitative values come from bound source.json, never authored into the spec. tools/render makes SVG/PNG graphics, an XLSX comparison workbook and previews. Choose genuinely distinct explanatory views.

Compute hashes only after final input selection. Run this definition’s absolute bin/check from the workspace. Production roles first invoke this definition’s tools/render; it reads declarative spec only. Source helpers and dependency runtime are host-controlled. A passed file check establishes structure and binding, not quality. An independent publication review covers every format and actual rendered previews before release.

Each decision_path has 3–4 steps, each exactly {heading,detail,fact_ids,message_ids}, heading at most 55 characters, detail at most 180 characters. Other visual kinds have steps: []. Keep titles under 100 characters, subtitle under 180, caption under 240 for legible graphics. Effects supports at most three inferential comparisons; choose another view for larger sets.
