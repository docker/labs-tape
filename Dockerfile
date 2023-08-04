ARG GOLANG_IMAGE=docker.io/library/golang:1.20@sha256:bc5f0b5e43282627279fe5262ae275fecb3d2eae3b33977a7fd200c7a760d6f1

FROM --platform=${BUILDPLATFORM} ${GOLANG_IMAGE} as builder-base

WORKDIR /src
ENV GOPRIVATE=github.com/docker

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
