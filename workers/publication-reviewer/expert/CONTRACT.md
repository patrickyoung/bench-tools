# Publication specialist contract

Inputs: request.json and all explicitly selected regular inputs/ files. Read inputs/source.json for authoritative facts; additional story/design files are selected by the controller. No prior-run discovery.

Output output/spec.json has exactly schema,role,request_sha256,inputs,content. schema is bench.publication-spec/v1. role is publication-reviewer. Bind SHA256 of exact request bytes; inputs is a path-sorted list of {path,sha256} for every regular selected input recursively, paths beginning inputs/. No symlinks, duplicate JSON keys or nonfinite values.

content has verdict (publish or revise), summary, rubric, findings, cross_format_checks (5-20 strings). rubric has exactly evidence_fidelity,narrative_coherence,audience_usefulness,writing_quality,visual_craft,accessibility, each integer1-5. findings is0-40 exact {severity,artifact,location,issue,correction}; severity material or minor. Publish requires every score>=4, no material findings and inputs/visual-review.json verdict pass. Visual critiques are actual controller-supplied image reviews; do not claim to have personally viewed attachments absent from your context. Review every production spec and its reader text against source and story. The quality threshold blocks delivery rather than silently labeling a draft excellent.

Compute hashes only after final input selection. Run this definition’s absolute bin/check from the workspace. Production roles first invoke this definition’s tools/render; it reads declarative spec only. Source helpers and dependency runtime are host-controlled. A passed file check establishes structure and binding, not quality. An independent publication review covers every format and actual rendered previews before release.
