# Guide

Context is the external-evidence seam in Bench. It is intentionally narrower
than a context platform: one catalogue, one connector invocation, one record
contract, and ordinary filters.

## A first source

Copy the example into a project-local connector directory:

```sh
mkdir -p .context/connectors
cp examples/connectors/handbook .context/connectors/handbook
chmod +x .context/connectors/handbook
context ls
context query handbook 'What is the incident channel?'
```

Replace the example's local lookup with the provider client. Keep the outer
`describe` and `query` contract unchanged. A Glean, LlamaIndex, Databricks
Genie, filesystem, SQL, or proprietary source is then the same program to the
caller.

The bundled Wikipedia connector is a complete network-backed example:

```sh
install -m 755 examples/connectors/wikipedia .context/connectors/wikipedia
WIKIPEDIA_USER_AGENT='my-context/1.0 (https://example.com/contact)' \
  context query wikipedia 'Unix filter design'
```

It is useful for general encyclopedic orientation, not as a replacement for a
primary or authoritative source where the decision is consequential. Its
records retain Wikipedia page identity, URL, modification time, license, and
attribution metadata so a later consumer does not lose where the snippet came
from.

## Who chooses sources?

The user or procedure does. Context never guesses.

- Put durable rules such as "use Genie for governed business metrics" or
  "search Glean before asking the user for an internal policy" in a Brief
  skill.
- Let the user name a source when their intent is explicit.
- If judgment is genuinely required, Ask can inspect `context ls` and choose a
  name. Record that choice in Ask like any other model decision.
- Keep authorization outside selection. Listing a source does not mean every
  user may see every object in it; the connector enforces source permissions.

This avoids a hidden broker whose routing changes answers without leaving a
visible decision.

## Multiple sources

Query each source independently, then merge the records:

```sh
context query glean "$q" > /tmp/glean.jsonl
context query genie "$q" > /tmp/genie.jsonl
cat /tmp/glean.jsonl /tmp/genie.jsonl | context merge > /tmp/evidence.jsonl
```

The separate commands make policy visible: a script can require both, accept
either, retry one, or stop on an authorization error. `merge` preserves source
order, removes byte-identical duplicates, and rejects two different records
with one citation identity. It does not rerank, summarize, or settle conflicts.
Those require task knowledge and belong in the consuming procedure or model.

For sensitive work, use a private temporary directory and remove it when the
run is complete. Better yet, stream a single source directly into the consumer
when aggregation is unnecessary.

## Brief, Ask, Ply, Agent, and Trail

Brief remains about procedure. A retrieval skill can name the sources to try,
the order, freshness expectations, and required citation behavior. It should
not contain provider transport code; that belongs in the connector.

Ask reasons over records:

```sh
context query handbook "$q" |
  ask 'Answer the question from these records. Cite every claim with its ref.'
```

The Context record is evidence, not a system prompt. Keep it in request input;
do not splice retrieved text into Brief instructions or `ASK_SYSTEM`. When Ask
records the request, its event history holds the exact snapshot and Trail can
find or display it later. Replay verifies that history instead of refetching
mutable external state.

Ply supplies iteration and acceptance. Put the single `context` executable in
its toolbox and expose only the intended connectors through `CONTEXT_PATH`.
Use a check when the outcome requires citations or a domain invariant. The
check can validate captured JSONL with `context check` and then inspect the
candidate report for the refs the task requires. Context deliberately does not
parse prose to decide whether a citation supports a claim.

Agent ties these focused programs to a durable goal. It need not gain one tool
per provider: it sees `context`, while the operator controls the connector
catalogue. Ask still owns model events, Brief the procedure, Ply the loop and
verifier, and Trail the read-only history. Context adds evidence provenance; it
does not replace any of them.

## When not to use Context

Use a direct program when the source is already a normal Unix input: `cat` for
a named local file, `git show` for a known commit, or `curl` for a simple public
endpoint. A connector earns its place when identity, retrieval, normalization,
or provenance varies by source and consumers benefit from one record contract.

Do not use Context as long-term memory, a vector database, an agent state
store, or a workflow engine. Use the system that already owns those facts and
put a connector at its boundary if retrieval needs to compose with Bench.
