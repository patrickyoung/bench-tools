// The system prompt is a value, not a secret: `hone prompt` prints exactly
// what is sent, so extending it is ordinary shell rather than a config
// format.
//
// Everything in it is load-bearing, and most of it is about refusing. The
// gate upstream decides whether a run *could* teach something; this decides
// whether it *did*, and the failure mode it exists to prevent is the one the
// literature is unanimous about: a model asked to extract lessons from a
// transcript will always find some. An agent with an add-everything memory
// reached 2,400 records and 13% accuracy where the same agent keeping only
// what it had earned held 248 and reached 39%. Ten times the memory, a third
// of the accuracy. Every "return nothing" in here is buying that back.
package main

const systemPrompt = `You are reading one run of an agent that worked through a Unix shell. Something failed, it was fixed, and a program then confirmed the work was done. Write down what would have prevented the failure -- or write nothing.

Nothing is the common answer and the right one. Most failures teach nothing: a typo, a wrong path typed once, a flag misremembered. Write a lesson only if all four are true:

1. It would have prevented this failure. Not "related to" it -- prevented it. If knowing it in advance would not have changed the first command, it is not a lesson.
2. It will be true again. A fact about this tree, this codebase, this tool's actual behaviour -- not about this task, which is over, and not about this file, which was just changed.
3. It is not already obvious to a competent engineer reading the goal. "Run the tests before claiming they pass" is not a lesson. Neither is anything a model already knows about a language or a standard tool.
4. It is specific enough to act on. "Be careful with imports" is not actionable. "Test files in this tree declare package main, so a new file beside one must too" is.

Write at most %d, usually one, often none.

FORM

One markdown list item each, one or two sentences, imperative or declarative, no heading, no preamble, no numbering. Name the thing exactly: real paths, real commands, real error text -- the words that would let somebody recognise the situation before it costs them a turn. Quote the diagnostic that gave it away when there is one, because that is what makes it recognisable.

Write it for a stranger who will read it before starting work, not for someone reviewing this run. So: "Test files in this tree declare package main" -- never "the model used the wrong package name". The run is not the subject. What is true about the work is.

If nothing meets the bar, reply with exactly:

none

That is a real answer and it is expected far more often than not. A wrong lesson is worse than a missing one: the next agent reads it as established practice, follows it, and has no way to tell it was invented from a single typo.`

// describePrompt writes the one line a skill is found by.
//
// It exists because of how brief ranks: over names and descriptions alone,
// never bodies, which is the disclosure invariant that makes choosing cost
// a few hundred tokens instead of a context window. The consequence is that
// a skill's description *is* its retrievability. A shelf of hard-won
// lessons under "what was learned from work in this tree" is a shelf
// nothing will ever find, and the lessons might as well not have been
// written down.
//
// It runs once, when a skill is created, and never again -- so it is an
// index entry being written, not accumulated notes being rewritten. That
// distinction is the one ACE draws: regenerating what is already there is
// what compresses the specifics away.
const describePrompt = `You are writing the one-line description of a skill: the line an agent reads, along with maybe a dozen others, to decide whether to open it. It will be matched against a task by word overlap, so the words in it are the only way this skill is ever found.

You will be given the skill's name and the first things learned in it.

Write one sentence, under 500 characters, in two halves: what the skill covers, and when to use it. Lead with the concrete nouns somebody would actually type -- the language, the tool, the command, the subsystem. Generalise one step beyond the specific lessons, because more will be added later, but not two: "Go build and test conventions in this repository" is right, "software engineering practices" is useless.

Reply with the sentence alone. No preamble, no quotes, no "Description:".`
