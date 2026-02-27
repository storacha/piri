# ============================================
# Build stage (shared)
# ============================================
# Use BUILDPLATFORM so Go runs natively (fast), while cross-compiling for TARGETPLATFORM
FROM --platform=$BUILDPLATFORM golang:1.25.3-trixie AS build

# Docker sets these automatically during multi-platform builds
ARG TARGETARCH
ARG TARGETOS=linux

WORKDIR /go/src/piri

COPY go.* .
RUN go mod download
COPY . .

# Production build - with symbol stripping
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} make piri-prod

# ============================================
# Debug build stage
# ============================================
FROM build AS build-debug

ARG TARGETARCH
ARG TARGETOS=linux

# Install delve debugger for target architecture
# Cross-compiled binaries go to /go/bin/${GOOS}_${GOARCH}/, same-platform to /go/bin/
# Normalize to /go/bin/dlv for consistent COPY path
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go install github.com/go-delve/delve/cmd/dlv@latest && \
    ([ -f /go/bin/dlv ] || mv /go/bin/${TARGETOS}_${TARGETARCH}/dlv /go/bin/dlv)

# Debug build - no optimizations, no inlining
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} make piri-debug

# ============================================
# Production image
# ============================================
FROM debian:bookworm-slim AS prod

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /go/src/piri/piri /usr/bin/piri

ENTRYPOINT ["/usr/bin/piri"]

# ============================================
# Development image
# ============================================
FROM debian:bookworm-slim AS dev

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    # Shell experience
    bash-completion \
    less \
    vim-tiny \
    # Process debugging
    procps \
    htop \
    strace \
    # Network debugging
    iputils-ping \
    dnsutils \
    net-tools \
    tcpdump \
    # Data tools
    jq \
    && rm -rf /var/lib/apt/lists/*

# Delve debugger
COPY --from=build-debug /go/bin/dlv /usr/bin/dlv

# Debug binary (with symbols, no optimizations)
COPY --from=build-debug /go/src/piri/piri /usr/bin/piri

# Shell niceties
ENV TERM=xterm-256color
RUN echo 'alias ll="ls -la"' >> /etc/bash.bashrc && \
    echo 'PS1="\[\e[32m\][piri-dev]\[\e[m\] \w# "' >> /etc/bash.bashrc

SHELL ["/bin/bash", "-c"]
ENTRYPOINT ["/usr/bin/piri"]
