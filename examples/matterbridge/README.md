# Persistent builder and deployment chats through Matterbridge

This optional application connects **Matterbridge** to the existing Bench
deployment commands. The first network provisioner is Telegram: use one
forum-enabled supergroup, one persistent builder topic, and one topic per
deployed worker or team. Omnigent is not required.

The implementation is prepared for operator setup. Offline checks exercise
message contracts, source packaging, the real Tend admission boundary and
failure handling. They do **not** establish a working Telegram account, a live
Matterbridge connection, native Docker confinement, or model-backed build
quality on your host. Run the live acceptance below before calling an
installation ready.

## The user experience

1. In the builder topic, send `/new worker release-reviewer` or
   `/new team research-team`, then describe the intended outcome, inputs and
   deliverables. Ordinary follow-up messages revise the selected draft.
2. Hire builds in a job sandbox. The candidate's source and the conversation
   are retained separately. Building does not deploy anything.
3. Send `/deploy release-reviewer` when you want that candidate installed.
   This explicitly authorizes installation and a model-backed smoke run.
4. The provisioner packages an immutable candidate, installs the independent
   tools and image, configures its queue, and runs its supplied smoke example.
   Only a successful smoke job allows topic creation.
5. The supervisor connects the new topic to Matterbridge, starts its service
   worker, checks the bridge API, and posts the topic link. Subsequent messages
   go to that deployment through its durable queue.

`/status` works while a job runs. `/cancel` requests cancellation; it does not
assert that execution or external effects have stopped. Unknown outcomes pause
the conversation until operator recovery. New requests in that topic remain
retained while other topics can continue.

The builder retains the selected candidate across requests and restarts. Each
worker topic receives its last 30 conversation entries, bounded to 128 KiB,
as explicit request context. Complete admitted messages and replies remain in
the application record. Previous job files are **not** implicitly attached to
new jobs. File uploads, voice messages and downloadable result attachments are
not implemented in this first adapter. Use the deployment's `read-file` command
for retained artifacts.

Generated teams are ordinary Agent-runnable assemblies: a manager definition,
reusable specialists, explicit handoffs and acceptance. This does not convert
arbitrary external team entrypoints or install every possible specialty
dependency. Missing dependencies must make the smoke run fail. A passing
structural check or smoke case is not a general claim about the team's quality.

## Setup on a local or remote execution host

For a remote machine, perform these steps **on that machine through SSH**.
The state, Docker socket and queue must all be local to that host. No shared
network filesystem, remote Docker mount, or multi-host scheduling is assumed.

The simplest first host is a dedicated Linux VM with Docker Engine. Keep the
small supervisor and independent Unix commands on that host; Docker isolates
the jobs. Persistent state stays on its local disk. The Telegram connection
is outbound, and the bridge API binds only to loopback, so chat needs no public
application listener. Size the machine for concurrent jobs: the default sandbox
limit is 4 GiB and two CPUs per running job. Start with a few deployments and
measure actual usage; there is no cross-deployment resource scheduler here.
See [Docker's installation guide](https://docs.docker.com/engine/install/ubuntu/).

Supply the variables listed in [environment.example](environment.example) to
setup and the supervised process using your service manager or secret store.
Only variable names are stored in application settings. The supervisor supplies
Matterbridge's two tokens through its environment, not TOML or command arguments.
The existing deployment service stores the selected provider profile privately
in its mode-0600 client file so queued jobs can execute independently. Restart
and rerun bootstrap to rotate the builder's provider profile; deployed profiles
can be rotated through their existing `service add-client` command.

The short setup path builds a pinned Matterbridge binary, installs the builder,
checks the Telegram group and creates the builder topic:

```sh
/srv/bench-tools/examples/matterbridge/setup --state /srv/bench-chat --ref FULL_REVIEWED_COMMIT
/srv/bench-tools/examples/matterbridge/bridge --state /srv/bench-chat run
```

Pass `--matterbridge /absolute/path/to/matterbridge` to reuse an installed
compatible binary. Run the same commands locally or in your SSH session; the
server remains running after you disconnect when its OS supervises `bridge run`.
The remaining sections document prerequisites, explicit configuration and
recovery rather than additional mandatory setup steps.

Prerequisites:

- Python 3.9+, Git, Go 1.26+, tar, and a healthy local Docker daemon.
- This Bench checkout and a full reviewed Bench commit. Source packaging is
  explicit; installing here never silently changes the source library.
- A Matterbridge binary with the Telegram `CHAT_ID/TOPIC_ID` mapping and API
  endpoints used here. Source reference inspected for this implementation:
  `42wim/matterbridge` commit `c4157a4d5b49fce79c80a30730dc7c404bacd663`.
  Install/build that separately following the upstream instructions. Do not
  assume an older release has the same topic support.
- A dedicated Telegram bot in a forum-enabled supergroup. It must be an
  administrator with **Manage Topics**. Only Matterbridge may poll this bot;
  do not run another `getUpdates` consumer or webhook for it.
- Environment variables for the Telegram bot token, a random Matterbridge API
  bearer token, and selected model settings, as shown in `environment.example`.
  Private credential files remain supported as an alternative.

Bots cannot create a new Telegram group using the Bot API. An administrator
creates the group once; this application creates its topics. **Topics are not
private access boundaries:** group members can see them. Use separate groups,
bots, state directories and credentials for unrelated clients. This first
application intentionally represents one collaborating group with a fixed
allowlist of Telegram user IDs and one selected provider profile. Display names
are never authorization.

Create a settings file outside the source checkout, replacing these paths and
IDs. Secret values stay in the selected environment, not in this settings file:

```json
{
  "bench_source": "/srv/bench-tools",
  "bench_ref": "FULL_40_CHARACTER_REVIEWED_COMMIT",
  "matterbridge": "/srv/bin/matterbridge",
  "api_url": "http://127.0.0.1:4242",
  "api_token_env": "MATTERBRIDGE_API_TOKEN",
  "telegram_token_env": "TELEGRAM_BOT_TOKEN",
  "telegram_chat_id": "-1001234567890",
  "allowed_user_ids": ["123456789"],
  "provider_env": ["ASK_MODEL", "OPENAI_API_KEY"]
}
```

Optionally include `builder_topic_id` to reuse an existing builder topic.
Alternatively, replace an `_env` selector with its `_file` equivalent and an
absolute path to a mode-0600 file. `provider_file` contains the JSON described
in [the deployment service guide](../omnigent/SERVICE.md). Select exactly one
environment or file source for each credential type.
Otherwise bootstrap creates one after checking the bot's group permissions and
installing the builder. Select an unused loopback API port. This application
manages **its own Matterbridge process and configuration**; it never rewrites
an existing shared Matterbridge configuration.

```sh
/srv/bench-tools/examples/matterbridge/bridge --state /srv/bench-chat init < /srv/private/chat-settings.json
/srv/bench-tools/examples/matterbridge/bridge --state /srv/bench-chat bootstrap
/srv/bench-tools/examples/matterbridge/bridge --state /srv/bench-chat run
```

`init` creates a new private state directory. `bootstrap` uses the existing
source packager and installer, with `--without-chat`, and checks Telegram
permissions; it makes no model call. `run` supplies process lifetime and chat
transport. Supervise it with your existing systemd/launchd/container supervisor.
The supplied [systemd example](bench-chat.service.example) is optional and uses
a pre-existing service account; adjust its absolute paths. No daemon is added
to the independent Bench tools.

Use `/new worker hello` and request a tiny text worker, revise it in a second
message, deploy it, then send two related messages in its topic. Verify that
the second request uses the first response and that a supervisor restart
retains the conversation. Confirm job and smoke outcomes with `service get`.
For a team, separately evaluate real member handoffs and its integrated result.
These live checks consume your configured model and create Telegram messages.

## Program boundaries and recovery

See [DESIGN.md](DESIGN.md) for the stream, file and lifecycle contracts.

```text
Telegram ↔ Matterbridge ↔ bridge receive/ingest/tick/flush
                               │
                         service submit/get
                               │
                              Tend → sandbox-job → Hire or Agent

explicit /deploy → provisioning Tend queue → provision
                     → package/install → smoke job → create topic
```

The bridge translates messages and retains conversation context. It does not
implement model clients, an action loop, worker scheduling or a second job
database. `run` supervises the Matterbridge process, the provisioning queue's
shell worker and one existing `service-worker` per deployment. A process exit
stops the supervisor for inspection; the OS may restart it. Tend retains jobs
and uncertainty across those restarts.

Inspect without executing another job:

```sh
/srv/bench-tools/examples/matterbridge/bridge --state /srv/bench-chat status
/srv/bench-chat/builder/package/service --state /srv/bench-chat/builder/queue list chat
```

Deployment IDs and delivery indexes appear in `bridge status`. For a failed
provisioning job, inspect its retained attempt output before asking Tend to
retry. For an unknown job, stop/reconcile any installer or smoke execution
before resolving it. Use the exact recorded queue:

```sh
TEND_ROOT=/srv/bench-chat/provision-queue /srv/bench-chat/builder/package/prefix/bin/tend show DEPLOYMENT_ID
TEND_ROOT=/srv/bench-chat/provision-queue /srv/bench-chat/builder/package/prefix/bin/tend events DEPLOYMENT_ID
```

The deployment's smoke record and queue live in
`deployments/DEPLOYMENT_ID/smoke.json` and `deployments/DEPLOYMENT_ID/queue`.
Use that package's `service recover` to stop its owned container before
releasing uncertain smoke or chat jobs. Merely retrying the provisioning queue
does not resolve a fenced sandbox job.

Telegram's topic creation has no idempotency key. A lost response leaves a
marker and prevents another create call. Inspect Telegram, then bind the
existing topic explicitly before retrying the failed provisioning attempt:

```sh
/srv/bench-tools/examples/matterbridge/bridge --state /srv/bench-chat attach-topic DEPLOYMENT_ID TOPIC_ID
```

Use `builder` instead of the deployment ID for uncertain bootstrap topic
creation. If the topic truly was not created, create it manually and attach
its ID. Existing completed deployments cannot be silently rebound.

Matterbridge's API accepts outgoing posts before the network confirms delivery.
A lost API response marks the outbox item unknown and never automatically
resends it. After inspecting the conversation, explicitly resolve it:

```sh
/srv/bench-tools/examples/matterbridge/bridge --state /srv/bench-chat resolve-delivery INDEX sent
# Or, only if a resend is intended:
/srv/bench-tools/examples/matterbridge/bridge --state /srv/bench-chat resolve-delivery INDEX retry
```

## Practical limits

- **Durability begins at local admission.** Matterbridge's incoming API is an
  in-memory, destructive buffer. A crash or lost response before local saving
  can lose messages; Tend cannot repair this earlier transport gap. The API
  should have exactly one consumer. A receive drain precedes configuration
  restarts, but does not eliminate that gap. There is no end-to-end exactly-once
  delivery claim.
- Matterbridge must restart to add a new topic mapping. Each gateway contains
  exactly one Telegram topic plus the API; replies never intentionally
  broadcast to other worker topics. A healthy local API and live process do
  not prove that Telegram received a particular post.
- Chat is text-only; the deployment service's selected-file interface remains
  available separately. Explicitly copy prior artifacts when a task needs them.
- Retention is bounded to 10,000 admitted messages (or a 12 MiB conversation
  record) and 20 deployment names per
  instance. The underlying service also retains its existing limits. Archive
  a stopped instance and start a fresh one when necessary. There is no automatic
  deletion, upgrade, deployment replacement or channel migration.
- Builds export up to 64 source files and 8 MiB; the serialized candidate must
  also fit the service's selected-file bound to support subsequent revision.
  Unsupported or excessive candidates are rejected, not silently truncated.
- Credentials and live state stay outside exported source. Back up the full
  selected state with processes stopped. Generated Matterbridge TOML contains
  empty token placeholders; only its selected process environment receives the
  values. The application state still contains private conversations and provider
  profiles and must not be published.

## References and checks

- [Matterbridge API](https://github.com/42wim/matterbridge/wiki/Api)
- [Matterbridge Telegram topic mapping](https://github.com/42wim/matterbridge/blob/c4157a4d5b49fce79c80a30730dc7c404bacd663/bridge/telegram/telegram.go)
- [Telegram topic creation](https://core.telegram.org/bots/api#createforumtopic)

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s scripts/tests -p 'test_matterbridge.py' -v
```

Tests use synthetic candidate sources and a loopback Matterbridge API fixture.
They do not contact Telegram or spend model credits. The existing deployment
integration suite separately exercises real Tend, Agent, MCP and SSH contracts.
