# Method provenance

Research notes checked **2026-09-22**; attributed primary-source methods below
were incorporated from the supplied research, not re-fetched during this build.
These are public design lenses, not company mandates, local approval, proof of
runtime consultation or guarantees of currently available products. Revalidate
volatile facts only through caller-authorized current sources. Select supported
versions per job; “latest” links do not freeze a contractual version.

- Microsoft, [Azure Well-Architected pillars](https://learn.microsoft.com/en-us/azure/well-architected/pillars):
  balance reliability, security, cost, operations and performance for the workload;
  vendor-specific lens, not enterprise policy.
- AWS, [Well-Architected](https://docs.aws.amazon.com/wellarchitected/latest/framework/welcome.html):
  explicit trade-offs include sustainability; review is not an audit.
- Simon Brown, [C4 diagrams](https://c4model.com/diagrams), CC BY 4.0:
  select useful context/container, dynamic and deployment views rather than all views.
- NIST, [SP 800-207](https://csrc.nist.gov/pubs/sp/800/207/final):
  resource and identity access decisions cannot rely on network location alone.
- OWASP, [ASVS](https://github.com/OWASP/ASVS):
  scope supported verification requirements and control IDs; do not infer certification.
- NIST, [AI RMF](https://www.nist.gov/itl/ai-risk-management-framework):
  voluntary trustworthiness/risk management throughout AI use; drafts or revision
  initiatives are not final company requirements.
- OpenTelemetry, [signals](https://opentelemetry.io/docs/concepts/signals/):
  traces, metrics and logs inform operation; instrumentation alone is no SLO or team.
- FinOps Foundation, [FinOps Framework](https://www.finops.org/framework/), CC BY 4.0:
  value, ownership and unit economics need business, engineering and finance;
  cost drivers are not fabricated prices.
- OpenAPI Initiative, [OpenAPI](https://spec.openapis.org/oas/latest.html):
  explicit HTTP contracts with platform-supported versions.
- AsyncAPI Initiative, [AsyncAPI 3.0.0](https://www.asyncapi.com/docs/reference/specification/v3.0.0):
  explicit message contracts; delivery guarantees and failure handling remain
  design responsibilities, not automatic properties of a schema.

Short guidance is original synthesis; no large source passages are reproduced.
MIT covers this definition's original work, not linked third-party materials.
