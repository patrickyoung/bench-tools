# One expert, many customer questions

A customer wants to export their data before closing an account. The policy
explains when exports are available, but says nothing about how long they
take. A useful reply must explain the first point and admit the second gap.

This starter is a folder, not a new agent application. Agent supplies the
runner, Brief reads its skill, Ask calls the model, Ply checks and corrects the
work, and Cite validates the reply's source references. Cage limits worker
writes and denies worker networking. There is no example runtime to maintain.

## Run the expert

From the monorepo root, install the companions and [configure a
model](../../docs/GETTING-STARTED.md#2-connect-a-model):

```sh
python3 scripts/install agent hire ask brief ply cage cite
export PATH="$HOME/.local/bin:$PATH"
practice=$(mktemp -d)
cp -R examples/support-reply "$practice/support-reply"
cd "$practice/support-reply"
mkdir customer-work
cp question.txt customer-work/question.txt
hire verify expert
agent show -C customer-work expert
```

The last two commands inspect structure and composition without model calls.
Read `expert/AGENTS.md`, its skill, policy records, and `expert/bin/check`.
Then run the job:

```sh
agent run -C customer-work -evidence customer-records -turns 8 expert -- \
  'Draft reply.md for the customer using the supplied policy.' > run-summary.txt
cat customer-work/reply.md
cat run-summary.txt
```

This uses your model account. The workspace contains the deliverable;
stdout contains the final report; stderr shows progress. A successful reply
should say that workspace owners can export CSV while their subscription is
active and should export before closure. It should say the policy does not
specify duration. Model wording varies. Nothing sends a customer message.

Agent keeps controller records in `customer-records/`, outside the mutable
workspace, and state in `customer-work/state/`. Default Cage confinement lets
actions write there and in their private temporary directory; host reads are
unrestricted. Run `cage check` if the host refuses the boundary.

## See what the check means

Inspect citation checking directly, without a model:

```sh
cite expert/sources.jsonl < customer-work/reply.md > checked-reply.md
printf '%s\n' '[ctx:demo:invented](https://example.com/policies/exports)' |
  cite expert/sources.jsonl
echo "$?"
```

The real reply should pass unchanged. The invented reference should produce
no stdout and exit **1**. Cite verifies reference identity, not truth: a reply
that invents a duration beside a valid link can still pass. That is why this
starter produces a draft for review. Add a stronger independent check for a
job that requires automatic acceptance of factual claims.

Repeating Agent against the accepted workspace may finish at the pre-check
without calling a model. For a different question, use a fresh workspace:

```sh
mkdir next-customer
printf '%s\n' 'Can I still export after closing my account?' > next-customer/question.txt
agent run -C next-customer -evidence next-records -turns 8 expert -- \
  'Draft reply.md for this customer using the supplied policy.'
```

The same expert serves both cases; each has its own work, state, and records.
Ply exit 2 means the run is unfinished, not an accepted reply. Inspect the
records and candidate before deciding how to continue.

## Build a different expert

Change this folder by hand, or describe the new job to [Hire](../../tools/hire/README.md).
Hire uses Agent to build another folder. Review its generated check just as
you reviewed this one. Keep deterministic transformations in ordinary programs
and put reusable judgment in instructions and skills.

Need only one draft, without file editing or correction turns? The
[evidence-answer starter](../evidence-answer/README.md) shows Context → Ask →
Cite. Need durable attempts? Submit the Agent command to
[Tend](../../tools/tend/examples/agent-checkpoint/README.md). The expert folder
and the runner do not need a new queue implementation.
