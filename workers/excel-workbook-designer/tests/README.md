# Text-only contract regression

Run `python3 workers/excel-workbook-designer/tests/test_spec.py` from the library
root. The synthetic fixture contains input/spec text only. The ten checks cover
valid binding and representative rejection paths; no model or Excel behavior is
claimed. They copy into temporary workspaces and leave source unchanged.

To test rendering, copy tests/fixture into an external workspace and move its
spec.json and guide.md into a new output/ subdirectory there. Then select the
WORKBOOK_* dependencies from the worker README, and invoke expert/tools/render
and expert/bin/check from that copy. Review previews and run separate independent
arithmetic and native-application checks. Real case records belong outside source.
