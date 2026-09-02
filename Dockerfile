# syntax=docker/dockerfile:1
#
# Builds and runs main.hive, from nothing but this folder.
#
#   docker build -t main .
#   docker run --rm -p 8080:8080 main
#
# Nothing has to be installed to build this but Docker: the image downloads Go
# — the one thing `hivec build` needs that it does not carry as source text the
# way it does its own runtime — downloads the compiler's latest release for the
# platform being built for, and compiles the program with the two of them.
#
# Two stages: the first has Go and the compiler and does the building; the
# second is just the binary that came out of it, with nothing else in the image
# — not Go, not the compiler, not even a shell.
#
# Everything beside this file goes into the build, so a .dockerignore is what
# keeps out of it what does not belong in it: a .git, a build directory, an
# .env holding anything private — which `docker run -e` hands over better
# anyway, and `hive.env.get` reads either the same way.
#
# Written by `hive container main.hive`, and an ordinary Dockerfile from there
# on: editing it is expected, and running the command again writes it afresh.

##############################################################################
# Stage 1 — download Go and the compiler, then build
##############################################################################
FROM debian:bookworm-slim AS builder

# What Docker is building for: amd64 on an ordinary machine, arm64 on an
# Apple-silicon one or a Graviton. Go and the compiler are both downloaded for
# it, so this builds natively wherever it runs, and
# `docker buildx build --platform linux/arm64 .` is the whole of building it
# somewhere else.
ARG TARGETARCH

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl \
 && rm -rf /var/lib/apt/lists/*

# Go itself, whatever the current release is.
RUN set -eux; \
	version="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1)"; \
	curl -fsSL "https://go.dev/dl/${version}.linux-${TARGETARCH}.tar.gz" -o /tmp/go.tar.gz; \
	tar -C /usr/local -xzf /tmp/go.tar.gz; \
	rm /tmp/go.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"

# The compiler itself, from hive-lang's latest release — named rather than
# versioned, so this build never has to be told what "latest" means and never
# goes stale as new releases ship.
RUN curl -fsSL \
      "https://github.com/R0DR160HM/hive-lang/releases/latest/download/hivec-linux-${TARGETARCH}" \
      -o /usr/local/bin/hivec \
 && chmod +x /usr/local/bin/hivec

# A static binary, so the runtime stage below needs no libc.
ENV CGO_ENABLED=0

WORKDIR /app

# The whole folder, because a program is its entrypoint and whatever that
# imported, and because what it reads while it runs is nobody's business but
# its own. A .dockerignore is what narrows this.
COPY . .

# `hivec build` leaves the executable beside the entrypoint, and the Go
# module it came from in main.hive-build/. The test is there to fail the image
# build now rather than at `docker run`.
RUN hivec build /app/main.hive \
 && test -x /app/main

##############################################################################
# Stage 2 — runtime
##############################################################################
# What the program ships in: the binary, the certificates an HTTPS call needs,
# the time zones — and nothing else at all. There is no shell in here, so
# `docker exec` has nothing to run; that is the point of it rather than an
# oversight.
FROM gcr.io/distroless/static-debian12 AS runtime
WORKDIR /app
COPY --from=builder /app/main /usr/local/bin/main

# 8080 is where this program serves: hive.net.httpServe(8080, ...) says so.
#
# EXPOSE documents that and nothing more — it opens nothing. Reaching the
# server from the machine running Docker takes `-p 8080:8080` on the run, and
# without it the container comes up, logs that it is serving, and answers
# nothing: it is listening on its own localhost, which is not the one the
# browser is pointed at. `docker ps` is where the two are told apart —
# `0.0.0.0:8080->8080/tcp` is published, a bare `8080/tcp` is only this line.
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/main"]
