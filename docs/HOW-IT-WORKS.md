# How the pieces work

[Home](../README.md) · [Tool guide](TOOLS.md) · [Recipes](RECIPES.md)

Suppose you want a weekly support digest. Some parts are already known: which
tickets to read, where to save the draft, and which fields must be present.
The part that needs judgment is deciding which themes matter and explaining
them clearly. Keep the fixed steps in ordinary code; give the model the
judgment step and the evidence it needs.

That separation makes failures easier to locate. A missing ticket is a data
problem. A poor summary is a model or instruction problem. A duplicate send
is an execution problem. Each calls for a different remedy.

## A program has more than one output

Most Bench commands use the operating system's three standard streams:

```text
                  instruction (command arguments)
                              |
input file -- stdin --> [ program ] -- stdout --> result file
                              |
                           stderr
                              |
                       progress or errors

The caller also receives an exit status: the program's outcome code.
```

For `ask 'Summarize this' < tickets.txt > digest.md`, the quoted words are
the instruction, `tickets.txt` supplies stdin, and `digest.md` receives stdout.
Stderr stays visible in the terminal. `a | b` connects A's stdout to B's stdin.

These are operating-system interfaces, so Python can use them too:

```python
result = subprocess.run(
    ["ask", "Summarize the important customer problems."],
    input=ticket_text, text=True, capture_output=True,
)
```

This fragment assumes `import subprocess` and a string named `ticket_text`.
Inspect `result.returncode` before using `result.stdout`; diagnostics are in
`result.stderr`. The [builder guide](BUILD-WITH-AN-LLM.md) gives a complete example.
Passing arguments as a list also keeps ticket text from becoming shell code.

Files make useful handoff points: save retrieved tickets once, review them,
try another prompt against the same input, and compare the outputs. When you
use a shell pipeline, remember that its default exit status usually comes
from the last command. Separate steps with checked statuses when an upstream
failure matters; a later command cannot infer missing input from an empty file.

## A model call becomes an agent when something executes its actions

**Ask** sends supplied messages to a model and retains the conversation. It
cannot run a command just because the answer contains one. It also cannot
read a file from its name. The caller supplies the contents.

**Ply** adds the feedback loop. It asks through Ask, executes a model-written
command, returns the observed output, and requests the next step. When the
model offers a final report, a check can accept it or return useful feedback.

```mermaid
flowchart LR
    G[Goal] --> C[Run caller's check]
    C -->|Passes| D[Done]
    C -->|Rejects| M[Ask model]
    M -->|Command| X[Execute and observe]
    X --> M
    M -->|Final report| C
```

The pre-check matters. If this week's digest is already complete, the caller
can return without another model turn. The feedback matters too: “missing an
owner for ticket 42” gives the model a specific problem to fix.

A check accepts with status 0, rejects with 1, and stops the loop as broken
with other statuses. A turn limit bounds how many times the model can respond.
Neither a turn limit nor a timeout proves that the result is good.

## Instructions, evidence, and authority answer different questions

| Question | The relevant piece | Support-digest example |
| --- | --- | --- |
| How should the work be done? | Brief reads a selected skill; Rules collects applicable instruction files | Use a concise format; preserve customer names |
| What facts should the answer use? | Context normalizes evidence; Ask receives it as message data | The actual tickets, source IDs, and retrieval time |
| Does a citation identify a supplied source? | Cite checks the answer against the saved evidence | A linked ticket must have the exact expected reference and URL |
| May this proposed change happen? | Action applies operator policy; May records human decisions | Review an exact proposal to post the digest |
| Where may a command write or connect? | Cage asks the OS to enforce write/network limits | Draft locally while denying action networking |

A skill is text the model is asked to follow. It is not a permission grant.
A ticket is evidence, even if its text tries to instruct the model. A tool
description advertises an operation; it does not authorize using it.

Likewise, putting a few commands in Ply's toolbox changes what is easy to
find on PATH. It does not prevent shell builtins or absolute-path execution.
Cage supplies an OS write/network boundary, but leaves host filesystem reads
available. Read isolation needs a separate environment designed for that purpose.

## Decide what “done” means before the model starts

Different checks support different claims:

| Evidence | What it establishes | What still needs checking |
| --- | --- | --- |
| A nonempty output file | Some bytes exist | Completeness and correctness |
| Valid JSON matching a schema | The required structure and types | Whether the values are true |
| Cite accepts a report | Citation identities satisfy its rules | Whether the source supports the prose |
| A test suite passes | The tested cases pass | Untested behavior and suitability for use |
| Ask replay verification passes | The retained history is internally consistent | Factual truth and whether history was truncated at a valid boundary |
| A service returns a matching operation receipt | Evidence about that external operation | Any remaining ambiguity under that service's contract |

For a digest, a useful check could verify that every input ticket is accounted
for, required sections exist, and cited IDs belong to the supplied dataset.
A reviewer still judges whether the themes and wording are useful. Choose
checks that reflect the task, not a convenient proxy such as file size.

If the model can rewrite the test that declares it done, a passing test is
weak evidence. Keep the acceptance definition under the caller's control.
This includes code the verifier imports or executes, not just the check's
filename. [Draft](../tools/draft/README.md) has a workflow for designing and
admitting a verifier; review what that verifier actually proves.

## Conversation memory and durable work solve different problems

An Ask session is a conversation record. A Ply checkpoint identifies the
current session to resume, including after compaction. Neither can tell you
whether an unacknowledged external request took effect.

**Agent** gives a recurring job a home: its goal, instructions, toolbox,
working files, check, and history. **Tend** queues and records attempts to run
an ordinary command. A scheduler or caller must invoke `tend work`; Tend does
one durable transition per invocation and exits. A queue alone does not keep
a worker process running.

**Weave** answers a different question: which tasks in a finite dependency
graph are ready, given the supplied observations? It prints those tasks. The
caller validates the observation evidence and passes ready work to an executor.

```text
Task definitions + observed outcomes --> Weave --> ready task records
                                                     |
                                        caller selects/submits a command
                                                     |
                                                    Tend
                                                     |
                                         one recorded command attempt
                                                     |
                                           caller checks the outcome
```

Imagine a digest was posted but the process died before saving the response.
The correct next step is to inspect the destination or use a service-supported
idempotency key, not assume the absence of a local success record means nothing
happened. Tend represents uncertain execution explicitly. Persistence gives
you recoverable evidence; it cannot make every external service exactly-once.

## Start small and add the missing responsibility

Use Ask for a summary. Add Brief when you want to reuse the method. Add Ply
when the next action depends on observed results. Add Agent when the job needs
a maintained home, Tend when attempts must be durable, and Weave when readiness
depends on other tasks. Keep deterministic work in ordinary programs.

The cost of this approach is that you own the connecting code and its checks.
An integrated coding assistant or workflow framework may provide more of that
application for you. The [comparison guide](COMPARISONS.md) explains that choice
using current tools and concrete examples.
