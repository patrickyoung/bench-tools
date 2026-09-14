# Carry evidence from source to answer

Context is the external-evidence seam in Bench. It is intentionally narrower
than a context platform: one catalogue, one connector invocation, one record
contract, and ordinary filters.

For a first run with no account, use the [local-record tutorial](README.md#first-make-a-small-evidence-file).
The source examples below run from Context's source directory:
`cd tools/context` from the monorepo root. Install Context first; model-writing
examples also need configured Ask and Cite.

For a complete offline composition with document and table evidence, a local
MCP service, and a separate task checker, run the
[Unix evidence example](examples/unix-evidence/README.md). It uses public
executables and the existing version 1 envelope.

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
mkdir -p ~/.context/connectors
install -m 755 .context/connectors/wikipedia ~/.context/connectors/wikipedia
WIKIPEDIA_USER_AGENT='my-context/1.0 (https://example.com/contact)' \
  context query wikipedia 'Unix filter design'
```

It is useful for general encyclopedic orientation, not as a replacement for a
primary or authoritative source where the decision is consequential. Its
records retain Wikipedia page and revision identity, an exact revision URL,
modification time, license, and attribution metadata so a later consumer does
not lose where the extract came from.

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

Here `glean` and `genie` stand for connectors you have installed, and `q` is
the question you want both to answer. Context does not bundle those services.

```sh
sources=$(mktemp -d)
context query glean "$q" > "$sources/glean.jsonl" &&
context query genie "$q" > "$sources/genie.jsonl" &&
cat "$sources/glean.jsonl" "$sources/genie.jsonl" | context merge > "$sources/evidence.jsonl"
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
q='What is the incident channel?'
context query handbook "$q" > evidence.jsonl &&
ask "Question: $q

Answer from these records. Cite every factual claim as an exact
[ref](citation.url) Markdown link." < evidence.jsonl > candidate.md &&
  cite evidence.jsonl < candidate.md
```

The question is deliberately present twice: Context needs it for retrieval and
Ask needs it as the task. A pipe carries Context's stdout, not Context's argv.
Leaving the question out of Ask asks the model to infer a task from evidence,
and an honest model may answer that no question was provided.

The Context record is evidence, not a system prompt. Keep it in request input;
do not splice retrieved text into Brief instructions or `ASK_SYSTEM`. When Ask
records the request, its event history holds the exact snapshot and Trail can
find or display it later. Replay verifies that history instead of refetching
mutable external state.

For normalized Context stdin, the `user` event also carries an evidence manifest
over the same bytes. It records the snapshot digest and location and the ordered refs,
sources, retrieval timestamps, citations, exact retrieval query, and connector
fingerprint. The content is not duplicated. `ask replay -check` rebuilds the
manifest from the message and rejects drift between the two.

Ply supplies iteration and acceptance. Put the single `context` executable in
its toolbox and expose only the intended connectors through `CONTEXT_PATH`.
Use `cite evidence.jsonl` as the final check when exact citation links are
required. It rejects unknown refs, mismatched URLs, and bare `ctx:` values and
prints the precise link expected. A task-specific check must still decide
whether a cited record actually supports a claim. Context deliberately does
not parse prose or own either judgment.

A direct `ask | cite` pipeline cannot put Cite's later exit status into an Ask
session: Cite is intentionally only a filter. If the verdict belongs in the
event history, use the same command as Ply's check. Cite keeps no session and
has no write access to Ask; Ply asks Ask to append a sealed `ply.verifier/v2`
record for every verifier execution while the candidate remains the preceding
assistant event.

Agent ties these focused programs to a durable goal. It need not gain one tool
per provider: it sees `context`, while the operator controls the connector
catalogue. Ask still owns model events, Brief the procedure, Ply the loop and
verifier, and Trail the read-only history. Context adds evidence provenance; it
does not replace any of them.

[Hire](https://github.com/patrickyoung/bench-tools/tree/main/tools/hire) builds
the reusable definition for that Agent. The
[support-reply example](https://github.com/patrickyoung/bench-tools/tree/main/examples/support-reply)
keeps a reviewed policy snapshot in the expert folder and uses a fresh
workspace per customer. It shows evidence reuse without adding a retrieval
platform or another worker loop.

## When not to use Context

Use a direct program when the source is already a normal Unix input: `cat` for
a named local file, `git show` for a known commit, or `curl` for a simple public
endpoint. A connector earns its place when identity, retrieval, normalization,
or provenance varies by source and consumers benefit from one record contract.

Do not use Context as long-term memory, a vector database, an agent state
store, or a workflow engine. Use the system that already owns those facts and
put a connector at its boundary if retrieval needs to compose with Bench.
