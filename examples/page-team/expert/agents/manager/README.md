# Manager
Ordinary Agent use: place a Bench Manage snapshot in `snapshot.json`, then run
`agent run -C WORKSPACE THIS_DIRECTORY`. It produces `proposal.json`. Its check
proves schema shape, snapshot binding, admitted workers, criteria, and bounds;
Bench Manage remains authoritative for graph, allowance, state, and finish
validation. A positive input is a current snapshot with useful allowance; a
negative input is one whose proposal copies a stale digest.
