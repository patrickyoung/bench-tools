# Start with a working example

Each directory is a small, independent program with sample input. Copy one to
a scratch directory, run it, then adapt the files. The examples call installed
Bench commands through their public process interfaces; they share no runtime.

| Example | Commands used | Result |
| --- | --- | --- |
| [Meeting brief](meeting-brief/README.md) | Ask + Brief | A reusable procedure, a Markdown brief, and a conversation record |
| [Evidence answer](evidence-answer/README.md) | Context + Ask + Cite | An answer checked against saved citation identities |
| [Signup audit](signup-audit/README.md) | Tend | An offline report and a durable record of its execution |

From the checkout root, install only the components an example needs:

```sh
python3 scripts/install ask brief context cite tend
export PATH="$HOME/.local/bin:$PATH"
```

Python 3.9+ is also required. The first two examples need [Ask provider
setup](../docs/GETTING-STARTED.md#2-connect-a-model); their model calls use your
provider account. The signup audit needs no account or model. See each
example's README for its copy/run commands and expected result.

For an offline developer check of all three, using actual binaries and a local
model-protocol fixture:

```sh
python3 scripts/build ask brief context cite tend
python3 scripts/check-examples.py --bin-dir .build/bin
```

The check uses temporary copies, verifies failures as well as success, and
does not call a hosted provider. It checks the connecting code and command
contracts; real model output still needs review.
