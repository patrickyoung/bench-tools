# Independent semantic acceptance rubric

Controller/reviewer owns this rubric; workers cannot weaken it. Review the exact
byte-bound package alongside selected evidence, not its confidence or a checker
pass. Record each criterion as satisfied, violated, or insufficient-evidence with
source/artifact locators, impact and owner/gate. No score averaging across hard
failures. Ready requires all material hard criteria satisfied; provisional may
retain explicitly bounded gaps with safe gates, never conceal them. Needs-input
is judged on honest blocking evidence and useful questions, not missing design.
Independently inspect for other defects and conflicting evidence too.

| ID | Required evidence / criterion | Good / bad contrast |
|---|---|---|
| Q-need | Request and journey trace to scoped outcomes, functional and measured quality requirements; current/target, constraints and assumptions separated | Capability enables outcome with validation owner / deployment falsely equals business success |
| Q-authority | Complete applicable EA pack/catalog/decisions mapped by authority/version/date/scope; failures assessed apart from scores; approved exception evidence in scope and current | Conditional gate for missing residency evidence / vendor “approved” or expired waiver treated as EA approval |
| Q-options | Credible reuse/configure/integrate, buy, build, process/no-op alternatives with simplest viable choice and evidence of suitability | Supported workflow configuration meets SSO/audit/retention / fashionable AI microservices skip existing platform |
| Q-trace | Requirements map to real components, interfaces, controls, ADRs, proof and owner handoffs; views agree with prose and JSON | SSO control and acceptance linked / decorative IDs or contradictory diagrams |
| Q-data | Ownership/classification/residency/retention and data flow/trust boundaries; relevant contracts and consistency/failure handling | Offline reconciliation and conflicts specified / unacknowledged duplicate or lost writes |
| Q-quality | Proportional load-conditioned SLO/performance/capacity, RTO/RPO, security/privacy, accessibility, support, cost and sustainability scenarios; targets labelled supplied/proposed | Proof method and owner, no tested claim / invented performance or capacity |
| Q-security | Identity/authz/tenant isolation, network boundaries, encryption/keys/secrets, threat/failure modes and verification scoped to risk | Named validation gate / “secure/compliant” without evidence |
| Q-delivery | Thin-slice proof acceptance, dependencies/owners, staged migration/reconciliation/cutover/rollback, CI/IaC/config/env, runbooks/alerts/support and exit conditions | Rollback includes data side effects and trigger / rollback merely “redeploy old version” |
| Q-evidence | Source qualifiers retained, uncertainty explicit, derived numbers reproducible; volatile facts authorized and current or unverified | Cost drivers/units plus validation / invented ROI, dates or commitments |
| Q-ai | When AI is considered, non-AI baseline, real need, authorized data, retention/grounding, injection/exfiltration controls, bounded tools, fallback, evaluation/monitoring, cost/latency and portability | Narrow tested advisory aid / autonomous agent gets business authority |
| Q-useful | Report concise, design coherent and proportional, diagrams legible and relevant, decisions and owners actionable | Context/flow/deployment views answer questions / massive boilerplate pack or empty boxes |

All are hard when applicable; visual styling preferences are not hard gates.
Use `not applicable` only with rationale, particularly for AI and scope-dependent
quality/interface concerns. Human approval, source entailment, pack completeness,
rendered diagram quality and actual operational behavior cannot be inferred by
the stdlib checker. Record unperformed render/tests explicitly.

## Independent evaluation recipes (not embedded job facts)
- Known-good family: approved workflow configuration with existing APIs,
  enterprise SSO/audit/retention, measurable proof and cutover/rollback handoffs.
- Near miss: polished autonomous AI microservices overlook reusable SaaS, invent
  ROI and treat mandatory residency as a preference under an expired exception.
  Shape/status rejection is mechanical; dishonest recommendation is semantic.
- Fresh transfer: industrial field workflow with offline connectivity, hybrid
  ERP synchronization, conflict handling and measured reconciliation. Different
  workloads must change the architecture, not just names on a cloud-only design.

Offline fixtures only test protocol. Controller runs fresh Agent cases and
independent review separately, preserving labels and these criteria. Do not
claim those live results from authoring tests, or tune the rubric to a candidate.
