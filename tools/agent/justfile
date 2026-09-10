check:
    sh -n bin/agent bin/agent-action-shell
    sh bin/agent_test.sh

eval:
    sh -n eval/run.sh
    sh eval/run.sh

install:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p ~/.local/bin
    ln -sfn "$PWD/bin/agent" ~/.local/bin/agent
    ln -sfn "$PWD/bin/agent-action-shell" ~/.local/bin/agent-action-shell
    echo "agent installed under ~/.local/bin."
