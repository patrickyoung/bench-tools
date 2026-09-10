# A filter-only example

These records demonstrate readiness, without running a model or executing work.
The observation is deliberately fabricated example data; its reference is not
evidence of a completed business task. A real adapter must verify actual results.

```sh
weave examples/quickstart/tasks.jsonl < /dev/null
weave examples/quickstart/tasks.jsonl < examples/quickstart/observations.jsonl
```

The first command prints `baseline`. The second prints `operations` and
`controls`, preserving each complete input record. `compare` remains blocked
until both reviews have verified accepted observations.
