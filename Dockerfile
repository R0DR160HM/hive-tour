# syntax=docker/dockerfile:1
#
# Builds and runs main.hive, from nothing but this folder.
#
#   docker build -t main .
#   docker run --rm -p 8080:8080 main
#
# Nothing has to be installed to build this but Docker: the image downloads Go
# — the one thing `hivec build` needs that it does not carry as source text the
# way it does its own runtime — downloads the compiler that wrote this file,
# for the platform being built for, and compiles the program with the two of
# them.
#
# Two stages: the first has Go and the compiler and does the building; the
# second is the binary that came out of it — and, because the tour has a
# playground, Go and the compiler again beside it. A Run button is a build:
# the reader's program is compiled here, for js/wasm, and run in their browser.
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

# The compiler itself — this one, rather than whatever release is newest on
# the day the image is built. The program beside this file was read by the
# compiler that wrote it, and that is the compiler that should build it: a
# later release is a different compiler, and an image following the newest is
# one that can stop building a program that never changed.
#
# `docker build --build-arg HIVEC_VERSION=v9.9.9 .` builds with another one,
# and running `hive container` from that compiler writes it in here.
ARG HIVEC_VERSION=v0.3.0
RUN curl -fsSL \
      "https://github.com/R0DR160HM/hive-lang/releases/download/${HIVEC_VERSION}/hivec-linux-${TARGETARCH}" \
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

# What of Go the playground's builds will never read, taken out before Go is
# copied into the image that serves: its own test suite, its API listings and
# its documentation. Everything `go build` uses stays.
RUN rm -rf /usr/local/go/test /usr/local/go/api /usr/local/go/doc

##############################################################################
# Stage 2 — runtime
##############################################################################
# What the program ships in: the binary, and what the playground builds with —
# Go, and the compiler under the name its own messages use, `hive`. main.hive
# serves Go's js/wasm loader out of this Go, so the loader a browser is sent
# always matches the toolchain that compiled the program it loads.
#
# It runs as a user of its own rather than as root: the server hands the
# compiler programs written by strangers, and nothing that compiler is asked to
# do needs anything a plain user does not have.
FROM debian:bookworm-slim AS runtime

RUN useradd --create-home --uid 10001 tour

COPY --from=builder /usr/local/go /usr/local/go
COPY --from=builder /usr/local/bin/hivec /usr/local/bin/hive
COPY --from=builder /app/main /usr/local/bin/main
ENV PATH="/usr/local/go/bin:${PATH}"
ENV CGO_ENABLED=0

USER tour
WORKDIR /home/tour
ENV HOME=/home/tour

# One build before the first reader's, to fill Go's build cache with the
# standard library compiled for js/wasm. Without it the first Run after every
# deploy compiles all of that while the reader waits — about ten seconds, and
# several hundred megabytes of memory more than a warm build takes. With it a
# build is about a second. The program reaches for the parts of the library
# the tour's programs use most, because the cache only holds what was built.
COPY --chown=tour <<'EOF' /home/tour/warm/main.hive
import hive.ui

type Bee {
	name: Str
	trips: Int
}

proc fly(bee: Bee): Int {
	hive.task.sleep(1)
	return bee.trips * 2
}

proc main(): void {
	bee := Bee("ada", 3)
	echo encode(bee, hive.json.codec())
	if Bee.decode("\{\"name\":\"grace\",\"trips\":4}", hive.json.codec()) is Result.Ok(other) {
		trips := await [fly(bee), fly(other)]
		echo trips
	}
	mut hive.map.Map<Str, Int> seen = hive.map.new()
	hive.map.set(seen, bee.name, bee.trips)
	echo hive.crypto.sha256(bee.name)
	echo hive.time.format(hive.time.now(), "%Y")
	echo hive.math.max(hive.conv.itf(1), 2.5)
	echo len(ui.html(ui.text([], "warm")))
}
EOF
RUN cd /home/tour/warm \
 && hive build main.hive --target js/wasm \
 && cd /home/tour && rm -rf /home/tour/warm

# 8080 is where this program serves: hive.net.httpServe(8080, ...) says so.
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/main"]
