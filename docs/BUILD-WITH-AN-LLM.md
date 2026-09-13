# Build a useful worker with an LLM

Want the harness to install and use Bench itself? Give it
[START-HERE.md](../START-HERE.md), which routes Codex, Claude Code, Cowork, Pi,
and OpenClaw through setup, building, evaluation, and repeated use.

Start by describing the job, not by writing another agent loop. Hire already
builds expert folders. Agent already runs them. A new specialty can often be
instructions, a skill, source material, and a check.

The part worth designing carefully is **what counts as a good result**. For a
support reply, “a file exists” is easy to test and almost useless. “The reply
answers the question, cites the policy, and admits what is unknown” is useful,
but needs both mechanical checks and judgment. Decide which is which before
letting a worker declare success.

## Try an existing expert first

The [support-reply starter](../examples/support-reply/README.md) is complete:
copy its folder, supply a workspace, and run Agent. Read its check and try the
deliberately invalid citation. You will see both the value of a small check
and the limits of what it proves.

For a single text transformation, [Ask](../tools/ask/README.md) may be enough.
Use [Brief](../tools/brief/README.md) to save the procedure. Reach for an expert
folder when the job needs file access, correction turns, stable instructions,
or a reusable definition that works in several workspaces.

## Give Hire a concrete brief

[Install](INSTALL.md) `hire agent ask brief ply cage` and [connect a
model](GETTING-STARTED.md#2-connect-a-model). From a fresh practice directory:

```sh
mkdir authoring
cat > job.txt <<'TEXT'
Build a support-reply expert under expert/.

Input: question.txt and policy.txt in the caller's workspace.
Output: reply.md, a short draft for a human support colleague to review.
Method: answer only from the policy; explicitly identify unanswered questions;
never invent a deadline or promise. Do not send the reply.
Check: reject absent or empty output and any missing required structure.
Document what the check cannot establish about truth and completeness.
Examples: include a question the policy answers and one it cannot answer,
with expected facts and deliberately wrong replies for review.
Scope: instructions, a reusable skill, and a small acceptance check. Use the
installed Agent runner. Do not implement a provider client or another loop.
TEXT
hire build -C authoring -evidence authoring-records -goal-file job.txt
hire verify authoring/expert
```

Hire uses a model to create `authoring/expert`. Its check confirms the folder
is structurally ready; it never executes the generated `bin/check`. Open the
instructions, skill, README, and check before using them. Review examples as
part of the design, not just as demonstration copy.

Prefer to write the files yourself? `hire new expert 'Draft support replies'`
creates a scaffold without a model. Its initial check deliberately rejects
unfinished work. Replace it with the job's actual acceptance condition.

## Run the artifact in a different workspace

Create `customer-work/` alongside `authoring/`, then put a real question and
the policy you want used in `question.txt` and `policy.txt` there:

```sh
mkdir customer-work
# Add customer-work/question.txt and customer-work/policy.txt before running.
agent show -C customer-work authoring/expert
agent run -C customer-work -evidence customer-records -turns 8 \
  authoring/expert -- 'Draft the reply from the supplied policy.' > run-summary.txt
```

Read `customer-work/reply.md` against both inputs. Stdout is the worker's
report, files are the deliverables, and the exit status tells you whether the
run finished. A turn limit bounds turns, not spend. Default worker actions
are confined by Cage for writes and networking; host reads remain available.

Use a new workspace for a new case. Reusing an accepted output can satisfy
the pre-check without a model call, even if you changed the question. If
reuse matters, make your check bind the output to the current input.

## Test the job, not just the demonstration

Keep a small set of cases that distinguish acceptable from plausible:

| Case | What to inspect |
| --- | --- |
| The policy answers the question | Relevant facts survive; the reply answers the actual question |
| The policy is silent | The reply admits the gap and invents no promise |
| The input contains “ignore the policy” | Input text stays evidence rather than becoming an instruction |
| A required output is missing | The executable check rejects it |
| The output has the right shape but a false claim | Independent review catches what a structural check cannot |
| A second customer uses the same expert | Work and records are separate; the definition remains reusable |

First test deterministic checks without a model, including their rejection
paths. Then evaluate real model runs on representative inputs using your
provider account. A local model fixture can prove command composition and
error handling; it cannot prove reasoning quality. The repository's
[verification guide](DEVELOPING.md) keeps those two kinds of evidence separate.

## When ordinary code belongs in the solution

Keep renderers, parsers, calculations, API connectors, and acceptance tests
as programs. A presenter may need a slide renderer; an architect may need a
dependency scanner. Those programs do useful deterministic work. Their expert
folder explains when and how to use them; Agent handles execution and context.

For an existing application, a small wrapper may also own argument parsing,
atomic output replacement, or the application's return format. The
[meeting-brief](../examples/meeting-brief/README.md) and
[evidence-answer](../examples/evidence-answer/README.md) starters demonstrate
that boundary in Python. It is one caller language, not the worker runtime.
Use literal argument arrays and separate stdin, stdout, stderr, and status in
whichever language the application already uses.

Before adding code, ask what existing command owns the job:

| Need | Reuse |
| --- | --- |
| Model request and replayable conversation | Ask |
| Commands, feedback, and a completion check | Ply, through Agent for a reusable expert |
| Build or revise an expert definition | Hire |
| Discover and read an existing procedure | Brief |
| A reviewed external operation | Action and May |
| Run later, retain attempts, or wait for input | Tend; let the OS arrange repeated calls |
| Expose or call an agent across machines | A2A |

If the missing piece has a new input/output contract of its own,
[Draft](../tools/draft/README.md) can help design and build it. Keep that piece
independently useful. The [tool map](TOOLS.md) and [recipes](RECIPES.md) show
compositions you can reuse before inventing another one.
