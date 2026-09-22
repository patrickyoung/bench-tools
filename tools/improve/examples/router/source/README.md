# Support router

Supply a JSON object containing a routing policy and ordered notes. The worker
returns a JSON array of id/queue objects. Run through Agent in a fresh workspace
containing request.json, with separate controller evidence.

bin/check validates raw JSON shape, allowed queues, IDs and order. It does not
establish routing correctness. The independent experiment scorer owns labels.
This deliberately naive definition is a teaching fixture, not a proven release.
