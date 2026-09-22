# Support note router

A deliberately naive keyword baseline for a small improvement experiment.
Reads a JSON request containing a policy and notes; returns only a JSON array
with one `{id, queue}` object per note in input order. Queues are security,
billing, technical and general. Python 3 is required for the structural check.
The caller puts the request at `request.json` and also supplies it on stdin.

Run with the public Agent command, selected model, fresh workspace and separate
evidence directory. Select `-require-action=false`; this job needs no tool action.
The checker validates shape and request IDs only. Independent labels outside
the definition measure routing quality. A passing check is not routing accuracy.

Example: `[{"id":"n1","queue":"general"}]` has valid structure for one note
whose ID is n1. Replacing its queue with sales must be rejected.
