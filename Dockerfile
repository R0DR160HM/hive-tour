# syntax=docker/dockerfile:1
#
# Builds and runs the Hive Tour from nothing but this repository: downloads
# Go (the one thing `hivec build` needs that it does not carry as source
# text the way it does its own runtime), downloads the compiler's latest
# release for Linux, compiles main.hive, and runs the server that comes
# out.
#
#   docker build -t hive-tour .
#   docker run --rm -p 8080:8080 hive-tour
#
# Two stages: the first has Go and the compiler and does the building; the
# second is just the binary that came out of it, with nothing else in the
# image — not Go, not the compiler, not even a shell.

##############################################################################
# Stage 1 — download Go and the compiler, then build
##############################################################################
FROM debian:bookworm-slim AS builder

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl \
 && rm -rf /var/lib/apt/lists/*

# Go itself, whatever the current release is.
RUN set -eux; \
	version="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1)"; \
	curl -fsSL "https://go.dev/dl/${version}.linux-amd64.tar.gz" -o /tmp/go.tar.gz; \
	tar -C /usr/local -xzf /tmp/go.tar.gz; \
	rm /tmp/go.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"

# The compiler itself, from hive-lang's latest release — named rather than
# versioned, so this build never has to be told what "latest" means and
# never goes stale as new releases ship.
RUN curl -fsSL \
      "https://github.com/R0DR160HM/hive-lang/releases/latest/download/hivec-linux-amd64" \
      -o /usr/local/bin/hivec \
 && chmod +x /usr/local/bin/hivec

# A static binary, so the runtime stage below needs no libc.
ENV CGO_ENABLED=0

WORKDIR /app
COPY main.hive ./main.hive

# `hivec build` leaves the executable beside the entrypoint and the Go
# module it came from in main.hive-build/. The test is there to fail the
# image build now rather than at `docker run`.
RUN hivec build /app/main.hive \
 && test -x /app/main

##############################################################################
# Stage 2 — runtime
##############################################################################
FROM gcr.io/distroless/static-debian12 AS runtime
WORKDIR /app
COPY --from=builder /app/main /usr/local/bin/hive-tour

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/hive-tour"]
