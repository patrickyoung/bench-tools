# Publication specialist contract

Inputs: request.json and all explicitly selected regular inputs/ files. Read inputs/source.json for authoritative facts; additional story/design files are selected by the controller. No prior-run discovery.

Output output/spec.json has exactly schema,role,request_sha256,inputs,content. schema is bench.publication-spec/v1. role is presentation-designer. Bind SHA256 of exact request bytes; inputs is a path-sorted list of {path,sha256} for every regular selected input recursively, paths beginning inputs/. No symlinks, duplicate JSON keys or nonfinite values.

content has title and slides (7-12). Every slide is exactly {kind,title,lead,body,fact_ids,message_ids,notes}. kind is cover, message, score_chart, comparison_table, tradeoff or decision. First is cover; include score_chart, comparison_table and decision. title <=90 characters; lead <=210; body is 0-4 short strings totaling at most75 words; notes <=1800characters. Cover every required story message. Charts and tables use exact source.json values automatically. tools/render creates editable native PPTX, PDF and slide previews. Vary composition to serve the narrative. Notes explain evidence and support the presenter, preserving caveats visibly when material.

Compute hashes only after final input selection. Run this definition’s absolute bin/check from the workspace. Production roles first invoke this definition’s tools/render; it reads declarative spec only. Source helpers and dependency runtime are host-controlled. A passed file check establishes structure and binding, not quality. An independent publication review covers every format and actual rendered previews before release.
