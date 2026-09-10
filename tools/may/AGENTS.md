# May contributor contract

`may` gives one human authority over one exact action. Keep that sentence
true.

- Read the action from stdin. Never accept it in argv, an environment
  variable, or a config file.
- Exit 0 only after a human approved those exact bytes, either at the current
  terminal or through a single-use grant for the named job.
- Exit 3 is a human refusal or the absence of a human. Exit 75 is a durable
  pending request. Exit 2 is usage or an operational failure.
- `request JOB` must expose the same consuming state transition as `may JOB`
  as one strict JSON object. It is a machine result, never another decision
  path; JSON fields or stdin may not approve anything.
- Bind job grants to the job and the exact action digest. Record the words as
  well as their digest. Consume grants by atomic rename before returning 0.
- Append JSONL audit records. Never edit or truncate the audit log.
- `decide` must read from `/dev/tty`; stdin belongs to action data and must
  never become an approval channel.
- No auto-approve flag, environment override, model call, classifier, daemon,
  network request, config file, or hidden database.
- Keep `may` and its state outside model toolboxes and writable sandboxes.
- Run `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build .`,
  and `./may check` before reporting success.
- Keep `may help`, `README.md`, and `may.1` consistent.
