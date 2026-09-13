# Start with a working example

Copy one directory to a scratch location, run it, then adapt the files. Start
with the expert folder to see a reusable digital worker; use a smaller filter
when one model request is enough. Each example uses installed Bench commands
through their public process interfaces.

| Example | Commands used | Result |
| --- | --- | --- |
| [Support reply](support-reply/README.md) | Agent + Brief + Ply + Ask + Cage + Cite; Hire inspects the definition | One reusable expert, separate customer workspaces, and a checked draft |
| [Meeting brief](meeting-brief/README.md) | Ask + Brief | A reusable procedure, a Markdown brief, and a conversation record |
| [Evidence answer](evidence-answer/README.md) | Context + Ask + Cite | An answer checked against saved citation identities |
| [Signup audit](signup-audit/README.md) | Tend | An offline report and a durable record of its execution |
| [Page team](page-team/README.md) | Agent + existing Bench Manage + Tend + Weave; MCP/A2A edges | Experimental creative team, checked artifact handoffs and a single HTML result |

From the checkout root, install only the components an example needs:

```sh
python3 scripts/install agent hire ask brief ply cage cite
export PATH="$HOME/.local/bin:$PATH"
```

That selection runs the support expert. Its runtime is the installed Go tools
and a small shell check. The other three examples show Python 3.9+ callers;
their READMEs list their smaller tool selections. The support, meeting, and
evidence examples need [Ask provider setup](../docs/GETTING-STARTED.md#2-connect-a-model)
and use your account. The signup audit needs no account or model.

The expert leaves its reply in the workspace and its report on stdout. The
two one-call writers replace their final output only on success. Tend retains
the audit's stdout as an attempt artifact. These are different output contracts;
each walkthrough shows exactly where to pick up the result.

The page team is the advanced assembly example. It has separate native creative
and browser prerequisites and uses the existing Bench Manage application. Read
its evaluation before running a paid creative case.

For an offline developer check of the starters and page-team contracts, using actual binaries and a local
model-protocol fixture:

```sh
python3 scripts/build ask brief context cite tend agent hire ply cage
python3 scripts/check-examples.py --bin-dir .build/bin
# On a host with a working Cage backend:
python3 scripts/check-examples.py --bin-dir .build/bin --native-cage
```

The check uses temporary copies, verifies failures as well as success, and
does not call a hosted provider. It checks the connecting code and command
contracts; real model output still needs review. The support case exercises
rejection and correction, two workspaces sharing one unchanged expert, replay,
and a passing pre-check with no extra model call. The default test explicitly
selects the host boundary; `--native-cage` exercises the example's default Cage.
