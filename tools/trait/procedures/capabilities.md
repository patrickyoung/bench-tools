
# Assess the complete capability automatically

The caller supplies the intended outcome. Choose and assemble the methods,
checks, executable helpers, native applications, selected existing interfaces,
and separate specialists needed to deliver it. Do not make the caller design
the file layout, select every tool, hand-write adapters, or author test cases.
Use the smallest complete implementation. A trait may require several of these
pieces. Document why each is used or unnecessary in CAPABILITIES.md beside
expert/, covering instructions, checks, programs/apps, specialists, dependencies,
and actual invocation/handoff contracts. Record missing prerequisites honestly.

Use ordinary Bench definitions. Write portable tools and meaningful checks,
preserve selected binaries/licenses, and document runtime and build requirements.
Inspect available public help. Do not download/install host dependencies, change
credentials, or fabricate successful capability use. Build native code only with
available compilers and writable temporary caches; retain source/build recipes.

For a single worker, Agent is the default entry. When the outcome requires a
team or deterministic application entry, assemble its actual external controller
and write evaluation/run.json beside expert/:

{"entry":"bin/run","args":["{goal}","{work}","{state}","{evidence}"],"requires":["python3"]}

The entry is relative to expert/. Args are literal tokens, never shell snippets.
Whole-token placeholders: {goal} is the current goal-file path, {work} the fresh
input/output workspace, {state} writable runtime state, {evidence} controller-owned
child records, and {expert} the definition path. Choose the real command's actual
argv; omit unused placeholders. Put any thin interface adapter inside expert/.
For stdin-goal commands, the controller also receives goal.md bytes on stdin.
Only use requires for actual external command names. Do not invent provider
clients, model loops, schedulers, or a new worker schema.

Trait runs team controllers on the host by default, as existing Bench teams do.
Keep all controller writes within its selected work/state/child-record/temp roots;
do not edit Trait source, criteria, outer records or results. This separation is
a contract, not host-mode confinement. The caller can optionally select a Cage
boundary, which enforces those write roots with full network/host reads, but some
hosts reject nested Cage. Never fall back or select permissions in run.json.
Child Agent actions should retain their default Cage; do not request -no-cage
or -net for those actions. Supply fixed limits from TRAIT_TURNS and TRAIT_TIMEOUT, model
from ASK_MODEL when selected, and Agent from TRAIT_AGENT. Use fresh child contexts,
explicit handoffs and -B; retain exact failure/unknown statuses and never blindly
retry. Every selected contribution must reach the final integration and check.

If an existing evaluation/run.json was explicitly supplied, preserve that
selection; adapt the requested worker to it or explain an incompatible request.
Training may teach use of existing capabilities but cannot change executable
contracts; it must preserve an existing runtime entry. Do not hide needed code
changes inside a teaching-only result.

Evaluation cases are generated separately when absent. Do not claim that your
description, structural validation, or a self-authored check proves useful work.

During authoring, use only mechanical/offline tests within the existing action
Cage. Never launch live team controllers or nested Agent/Ask work there: that
network boundary is intentionally closed. Trait runs live entry evaluation as
a separate stage after authoring. Use $TMPDIR or workspace paths for temporary
programs and compilation caches.
