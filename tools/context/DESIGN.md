# Design

Context concentrates provider variability without concentrating intelligence.
That distinction keeps the program aligned with the rest of Bench.

## The four steps

The whole program is four steps: find the named connector, give it the query,
validate and stamp its records, print them. Listing calls the connector's
description instead; merging applies the same record validation to a stream.

Nothing in those steps chooses a source or writes an answer. Selection requires
purpose and policy; synthesis requires task judgment. Brief and Ask already own
those jobs. Putting either into Context would create a second, less visible
agent.

## Pike and Thompson

The design follows the Unix approach described in Bell Labs' history of the
[Unix philosophy](https://www.nokia.com/bell-labs/unix-history/philosophy.html):
small programs, standard streams, and composition. Ken Thompson described the
system's success in terms of interfaces that let programs work together in his
[2003 interview](https://www.cs.princeton.edu/courses/archive/spring03/cs333/thompson).
Rob Pike's [Notes on Programming in C](https://zoo.cs.yale.edu/classes/cs223/doc-f2016/Pike.pdf)
argue for simple interfaces and data structures over clever machinery.

Those ideas show up here as concrete constraints:

- a connector is a program, not a plugin loaded into the Context process;
- JSONL is the common stream, not a shared provider object model;
- connector discovery is a path, not a registry service;
- `query`, `merge`, and the eventual answer are separate operations;
- explicit source names beat invisible routing;
- stdout is data, stderr is commentary, and exit status is outcome;
- a provider can use its best-supported language behind the executable seam.

The architecture is therefore decentralized in code but consistent in data.
Adding a source adds one program. It does not add a tool schema to every agent,
a dependency to every language, or a branch to the core.

## Why one envelope

Consumers need a few facts to compose evidence safely: source identity, stable
item identity, observation time, content type, a human title, and a locator for
review. That is the envelope. `content` stays open JSON because a table is not
a bad document and a metric is not a text chunk. Provider extensions survive
so the common contract does not erase useful precision.

The stable `ref` is derived at the boundary rather than trusted from model
text. It is a citation handle, not a claim of truth. Retrieval proves where a
record came from; a task-specific check decides whether the evidence is fresh,
authoritative, and sufficient.

## Deliberate omissions

There is no automatic router, federation planner, relevance ranker, reranker,
model, prompt, answer synthesizer, cache, index, database, daemon, server,
credential store, provider SDK, workflow, or session log in Context.

Some may be useful systems. They stay separate because they change at different
rates and have different owners. A LlamaIndex application may implement one
connector; Glean may implement another; a Genie connector may return tables.
Context makes their results composable without pretending their capabilities
are identical.

The [portable envelope](ENVELOPE.md) is documented independently of its
transport. MCP adapters remain ordinary executables at the edge; task checks
remain separate filters. The [offline example](examples/unix-evidence/README.md)
demonstrates both boundaries without adding protocol or policy to the core.

If repeated usage proves that explicit multi-source scripts are too awkward, a
small fan-out program can be added beside Context. Evidence should precede that
addition. It should not turn `query` into a broker whose selection, retries,
ranking, and synthesis are impossible to inspect independently.
