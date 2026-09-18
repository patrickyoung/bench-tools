# The resulting image ID is frozen in installed.json. Pin these base images
# by digest in a reviewed adaptation when reproducing the build byte-for-byte.
FROM golang:1.26-bookworm AS build
RUN apt-get update && apt-get install -y --no-install-recommends python3 && rm -rf /var/lib/apt/lists/*
WORKDIR /src
ADD bench-source.tar /src/
COPY build-tools.py /build-tools.py
COPY deployment.json /deployment.json
RUN python3 /build-tools.py /src /opt/bench --deployment /deployment.json

FROM node:22-bookworm-slim AS node
FROM python:3.12-slim-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates git poppler-utils && rm -rf /var/lib/apt/lists/*
COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=build /opt/bench /opt/bench
COPY definition /definition
RUN if [ -f /definition/expert/agents/analyst/requirements.txt ]; then \
      pip install --no-cache-dir -r /definition/expert/agents/analyst/requirements.txt; \
    elif [ -f /definition/expert/requirements.txt ]; then \
      pip install --no-cache-dir -r /definition/expert/requirements.txt; fi
COPY sandbox-agent /sandbox/bin/agent
COPY sandbox-builder.py /sandbox-builder.py
ENV PATH="/sandbox/bin:/opt/bench/bin:${PATH}" \
    BENCH_PREFIX=/opt/bench PYTHONDONTWRITEBYTECODE=1 \
    HOME=/tmp ASK_DIR=/job/evidence/ask POLARS_PYTHON=/usr/local/bin/python
WORKDIR /job/work
# The deployment launcher selects the actual host uid/gid, mounts and limits.
USER 65534:65534
