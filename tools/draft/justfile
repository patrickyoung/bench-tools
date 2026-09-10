# draft — what a human types.
#
# just is the caller, not a capability format. Recipes here are for people;
# anything a *model* has to call stays an executable in bin/, because -t
# makes PATH the toolbox alone and `just` is not on it. Putting `just` in a
# toolbox would hand the model every recipe at once and lose the aiming that
# is the whole point of -t.
#
# Each recipe below is a single shebang recipe rather than a list of lines,
# because just runs each line in its own shell otherwise and `cd` would not
# carry.

_default:
    @just --list

# run the check draft's own DESIGN.md names
check:
    #!/usr/bin/env bash
    set -euo pipefail
    export PATH="$(go env GOPATH)/bin:$PATH"
    sh -n bin/draft
    sh bin/draft_test.sh
    brief lint -strict skills/draft

# regenerate the tool reference from the installed binaries
sync:
    #!/usr/bin/env bash
    export PATH="$(go env GOPATH)/bin:$PATH"
    ./bin/draft sync

# install the four tools, then draft and its skill
install:
    #!/usr/bin/env bash
    set -euo pipefail
    for p in ask brief ply hone; do
        (cd "../$p" && go install .)
    done
    mkdir -p ~/bin ~/.claude/skills
    ln -sfn "$PWD/bin/draft" ~/bin/draft
    ln -sfn "$PWD/skills/draft" ~/.claude/skills/draft
    export PATH="$(go env GOPATH)/bin:$PATH"
    ./bin/draft sync
    echo "draft installed. Ensure ~/bin is on PATH."

# is a design buildable? DIR defaults to draft's own
verify dir=".":
    #!/usr/bin/env bash
    export PATH="$(go env GOPATH)/bin:$PATH"
    ./bin/draft check "{{ dir }}"

# scaffold a new design: just new hn "pull the YC feed and report on it"
new dir description="":
    #!/usr/bin/env bash
    export PATH="$(go env GOPATH)/bin:$PATH"
    ./bin/draft new "{{ dir }}" {{ if description == "" { "" } else { quote(description) } }}
