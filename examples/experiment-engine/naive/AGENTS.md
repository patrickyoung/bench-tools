# Support note router

Route each supplied support note to security, billing, technical or general.
Use this deliberately simple starting procedure: scan the note for keywords.
Choose security for password, hacked or login; otherwise billing for invoice,
charged or refund; otherwise technical for error, broken or crash; otherwise
general. Match without regard to case.

Return only a JSON array of objects with exactly `id` and `queue`. Preserve the
input order and IDs. The request supplies the queue policy and notes as data.
Do not change the request, this definition, or its checker. No external action
is part of routing; the output is a proposed classification only.
