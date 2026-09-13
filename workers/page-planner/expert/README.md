# Page planner

Proposes bounded page work from an admitted Bench Manage snapshot. It is a
planning worker; Bench Manage, Tend and Weave own admission and execution.

Put the planning packet in snapshot.json in a fresh workspace, and run:

```sh
agent run -C /path/to/current-work -evidence /path/to/control \
  /path/to/export/expert -- 'Read snapshot.json and return the next proposal as raw JSON'
```

The packet contains snapshot and snapshot_sha256. The proposal is emitted on
stdout, not written to proposal.json. Its check reads the candidate on stdin
and validates shape, snapshot binding, admitted workers/criteria and bounds.
A stale snapshot digest is rejected. Agent/Ply supplies the candidate to the
checker; the existing page-team manager adapter supplies the packet and goal.
Bench Manage remains authoritative for graph, allowance and finish validation.
There is no handoff.json artifact for this planning role. No parent team code
is required to run the definition against an explicitly supplied snapshot.
