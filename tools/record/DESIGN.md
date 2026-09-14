# An observation boundary

Record wraps an ordinary executable. Ask owns the only durable session format.
No tool imports another tool. Record remains an independent Unix program;
Agent enables it by default through Ply's explicit recording seam.
One new session belongs to one invocation. This keeps Ask's writer lock out of
the child's own conversation and makes missing terminal evidence unambiguous.

Three typed notes define version 1: `record.intent/v1`, `record.chunk/v1`, and
`record.terminal/v1`, all attributed to `record`. Ask seals every note. Intent
declares argv, physical cwd, selected executable hash, private FD numbers,
optional labels, and selected artifacts. Chunks bind a stream, byte offset,
and base64 bytes. Terminal binds every declared stream's length and SHA-256,
startup status, exit/signal, input delivery, EOF, interruption, and completeness.

Capture retains output before publishing it, without a presentation limit.
Ask append failures stop the child group and leave an incomplete session.
The stdin writer never holds the finalization mutex while reading upstream;
an upstream pipe cannot keep an exited child alive. Finalization closes input
and excludes later observations before recording the terminal outcome.

Replay uses Ask's verified snapshot, spools decoded streams to private files,
and validates every commitment before emitting data. It never reads original
artifacts. Child-session snapshots are also verified through Ask. Unknown
record versions, chunks after terminal, duplicate intents, missing streams,
incorrect offsets, and incomplete outcomes fail closed with status 125.

This records observation, not authority. It does not adjudicate approvals,
provide confinement, attest an executable, or establish business completion.
Effects may exist whenever execution could have started. Re-execution is a
separate caller action and is never part of replay.
