# Source provenance

New Polars/statistical expertise was built with Bench Hire after searching worker
and team catalogs (including experimental definitions) and finding no analyst.
Evidence discovery is reused unchanged from Product Owner at source pin
`a9c37fc43c958712637acf6db416456f171946bc`. The MIT license is retained.
Host-reviewed deterministic input/output routines and synthetic source tests add
only missing analytical contracts; Agent/Hire/Ask remain the execution programs.

Dependency versions in requirements.txt were installed and verified in a separate
operator environment. The renderer records actual loaded versions on every run.
Use the selected pins for repeatable rendering. Do not install during Agent work.

Library behavior was checked against official documentation:

- [Polars missing data](https://docs.pola.rs/user-guide/expressions/missing-data/)
- [Polars CSV scan](https://docs.pola.rs/api/python/stable/reference/api/polars.scan_csv.html)
- [SciPy independent t-test](https://docs.scipy.org/doc/scipy/reference/generated/scipy.stats.ttest_ind.html)
- [SciPy paired t-test](https://docs.scipy.org/doc/scipy/reference/generated/scipy.stats.ttest_rel.html)

Polars performs data preparation and descriptive statistics. SciPy performs
reviewed two-sided Welch/paired t inference; a small inspected helper applies
Holm's step-down adjustment across the full declared inferential family. The
method specification is explicit in CONTRACT.md. Tests verify effects, variances,
degrees of freedom and p/interval values with independent stdlib arithmetic and
Student-t density quadrature, plus auditing/refusal and byte-binding challenges.
Actual evaluation records and results are external to reusable source.
