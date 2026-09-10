# Model readiness before the first Bench model call

The Claude interface login, a model name, and a working Bench provider route are
three different facts. Establish the provider route immediately after
**SUITE-READY** and before any Bench contract, Draft, or Agent operation that
can call Ask.

## Bounded readiness probe

Inspect `../scripts/probe-model-access.sh`, then run its read-only plan with the
physical Ask executable from the pinned suite, one non-secret provider/model
identifier, and a private controller-evidence root outside every Agent home:

```text
probe-model-access.sh plan \
  --ask /absolute/prefix/lib/bench-suite/0.13.0/bin/ask \
  --model provider/model \
  --out /absolute/controller/model-readiness
```

The plan discloses one provider call, a 60-second timeout, evidence writes, and
the exact resume command. The approved `run --approve` uses Ask 0.2.0 from the
exact checksummed suite, an empty system prompt, no attachments, no tools, and a
constant-string JSON schema. It does not inspect, print, or retain credential
values or provider error bodies. Each attempt is retained privately, so the
same command can be retried after the access owner repairs the boundary.

A literal exit-zero schema-conforming response earns **MODEL-READY** only for
that provider/model in that process lane at that time. It does not prove model
quality, an Agent result, future availability, another account, or access from a
fresh launchd, systemd, CI, or Cowork session. A schedule must repeat an
equivalent probe in its fresh execution identity before activation.

## Failure and resume

On failure, preserve the helper's literal status and
**MODEL-ACCESS-REQUIRED** result. Record the non-secret model identifier,
resource/account identity, access and revocation owners, earliest blocked
model-backed command, and exact resume command. Do not print environment values,
ask for a key in chat, put a key in the Agent home, or mistake the Claude login
for Ask authentication.

Agent's public run command does not currently carry Ask's one-child OAuth
header-file-descriptor seam. An ordinary inherited environment credential may
be readable by model-authored actions because Cage is not a complete secret or
read boundary. A bounded low-risk interactive pilot may use an
organization-approved controller identity, but authenticated unattended work
remains **STEWARD-REQUIRED** until a reviewed credential mediator or stronger
outer task isolation proves delivery, non-disclosure, rotation, revocation, and
fresh-process behavior.
