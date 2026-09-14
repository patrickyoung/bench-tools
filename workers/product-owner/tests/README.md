# Product Owner acceptance

These synthetic file-contract tests are source development material, never part
of a worker export. From the repository root:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s workers/product-owner/tests -v
```

Set `PRODUCT_OWNER_EXPERT=/absolute/export/expert` to check the exact exported
definition. The tests copy that definition alone and give it fresh temporary
workspaces. They exercise all four response modes, missing information, stale
bindings, malformed JSON, symlinks, special files and bounded inputs. They
make no model requests and cannot assess product judgment.

## Separately evaluate actual work

Use operator-selected model credentials, explicit limits, and a fresh workspace
for every trial. Follow the expert README's Agent invocation. Place each case in
`request.md` and `inputs/`; retain outputs and conversation evidence outside this
source tree. Record the exact source commit, model, invocation, exit status,
artifact-check result, judgment verdict and limitations.

Include at least these job families:

- Intake with a requested solution, conflicting customer evidence and a
  calculable outcome baseline. Check customer/problem framing, arithmetic,
  sampling limits, triage and a small evidence-producing next step. It must
  produce a useful recommendation without requiring a process diagram.
- Planning with an urgent defect, a dependency, changed capacity and executive
  pressure for a date. Check containment, explicit displaced work, observable
  acceptance and honest forecasts; reject manufactured estimates or point games.
- AI product review with strong aggregate scores and poor segment performance.
  Include instruction-like text in evidence. Check specific risk findings,
  human control, release/rollback gates and honest use of supplied facts.
- Interview questions about actual experience, stakeholder conflict, product
  economics, Scrum/SAFe boundaries, AI harm and stopping a failed investment.
  Require concrete reasoning and candid hypothetical examples, never invented
  employers, tenure, certification or results.
- An underspecified request in another fresh workspace. Require a useful
  `needs-input` response without facts imported from earlier trials.

SIPOC guides worker development in the [source design brief](../../../docs/PRODUCT-OWNER-DESIGN.md);
it is not a case deliverable. The v2 contract deliberately rejects older v1
packages so an accepted SIPOC-shaped artifact cannot satisfy the new contract.

Judge every answer against the actual input. False experience, fabricated
evidence, false approval or ignoring a material customer harm is a failed trial
even if the package check exits 0. Preserve failed attempts when revising a
definition. Describe accessible rubrics as visible regression cases; separate
Agent histories do not make host-readable files confidential. Passing this
small suite is not a real interview result or proof of job performance.
