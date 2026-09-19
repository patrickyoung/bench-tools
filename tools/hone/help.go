package main

// Help fits eighty columns and goes to stdout, so `hone help | less` works
// and a misuse still leaves stdout empty for whatever was parsing it.
const usageText = `hone -- read a run a program judged, write down what it teaches

  hone [flags] [session ...]     distil what a run teaches
  hone forget <id> <skill>...    remove what a run taught
  hone show <proposal>            show an exact prepared skill document
  hone admit <proposal>           admit it without another model call
  hone prompt                    print the system prompt that words a lesson
  hone version                   print the version (-V, --version)
  hone help                      print this summary (-h, --help)

With no session, the current ask conversation. A directory means every
session in it, oldest first, which is what a batch over an archive wants.

the rule: hone learns from recoveries. A run that never failed teaches
nothing -- it needed nothing. A run that never passed proves nothing -- the
last thing tried might be wrong, and a wrong lesson is worse than none,
because the next agent reads it as established practice. Only a run that
failed and then passed teaches, and the lesson is the difference. That gate
is arithmetic on exit statuses already in the log; a model is used to word a
lesson, never to decide there is one.

the verdict: ply -check writes each result as a typed, sealed receipt in the
session, so hone knows how a run ended without guessing. Legacy signed prose
notes still read. A run with no check has no verdict and is refused, saying
so. Content repairs require replay-verified, changed assistant candidates
bound to rejected and accepted v2 receipts under the same check identities.
An empty pre-check or an unchanged answer cannot supply a content repair.

no store: a lesson is a skill, brief is the catalogue, and $BRIEF_PATH is
where it lives. hone writes what brief reads and stops -- there is no
index, no database, no embedding, and no daemon. Retrieval is brief find.

  hone                                  what did the last run teach?
  hone -into go-conventions             ...and keep it
  hone -into -                          ...into the skill the run followed
  hone -why                             replay-check and show evidence, no model
  hone -into house -prepare p.json run  prepare exact bytes for later review
  hone show p.json                      inspect those exact skill bytes
  hone admit p.json                     replay, stale-check, and write them
  hone -into house ~/.ask/sessions      everything, into one skill
  hone forget 20260801-2304-a3f9 house  that run taught something wrong

Flags come before sessions: after one, a word is a filename.

flags:
  -into skill   fold the lessons in instead of printing them; a name brief
                knows, or a path. A skill that does not exist is created.
                -into - means the skill the run was following (ply -s),
                which is where a lesson learned despite a procedure belongs
  -n N          most lessons from one run (default 3). Small on purpose
  -m spec       provider/model for the wording, e.g. anthropic/claude-sonnet-5
  -N            say what would be learned, write nothing
  -why          replay-check and print the evidence, then stop; no model
  -prepare file word one verified session into a user-named exact proposal;
                require -into, write no skill, and never overwrite the file
  -d dir        session directory ($ASK_DIR)
  -no-verify    skip replay for command recoveries; content needs verification
  -q            no progress on stderr

environment:
  ASK           the ask binary (default: ask on PATH). Required
  BRIEF         the brief binary (default: brief on PATH). Only for -into
  ASK_DIR       where sessions live (default ~/.ask/sessions)
  BRIEF_PATH    where skills live; hone writes to the first entry
  HONE_DIR     where hone keeps the sessions that worded its lessons
                (default ~/.hone/lessons)

exit: 0 something was learned · 1 nothing to learn · 2 error

1 is an answer, not a failure -- most runs teach nothing, so a loop over an
archive branches on it rather than stopping:

  for s in ~/.ask/sessions/*.jsonl; do hone "$s" -into house || continue; done
`
