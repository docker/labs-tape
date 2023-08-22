ARG GOLANG_IMAGE=docker.io/library/golang:1.21@sha256:b490ae1f0ece153648dd3c5d25be59a63f966b5f9e1311245c947de4506981aa

FROM --platform=${BUILDPLATFORM} ${GOLANG_IMAGE} as builder-base

WORKDIR /src

RUN --mount=type=bind,target=/src --mount=target=/root/.cache,type=cache --mount=target=/go/pkg/mod,type=cache \
    go vet ./...

RUN --mount=type=bind,target=/src,readwrite --mount=target=/root/.cache,type=cache --mount=target=/go/pkg/mod,type=cache \
    go test ./...

FROM builder-base as builder-arch

ARG TARGETOS
ARG TARGETARCH

RUN mkdir -p /out/${TARGETOS}/${TARGETARCH}/usr/bin

RUN --mount=type=bind,target=/src --mount=target=/root/.cache,type=cache --mount=target=/go/pkg/mod,type=cache \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
        go build -ldflags '-s -w' -o /out/${TARGETOS}/${TARGETARCH}/usr/bin/tape ./tape

FROM scratch

ARG TARGETOS
ARG TARGETARCH

# TODO: include CA certificates
WORKDIR /var/lib/buildkit-operator
COPY --from=builder-arch /out/${TARGETOS}/${TARGETARCH}/usr/bin /usr/bin

USER 65534:65534
ENTRYPOINT ["/usr/bin/tape"]
