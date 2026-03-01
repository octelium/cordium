
FROM golang:1.25.7 as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN make build-supervisor
RUN make build-workspace
RUN make build-workspace-git-cred-helper
RUN make build-cli-octelium
RUN make build-cli-octeliumctl


FROM debian:12.2-slim
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
  && apt-get -y install tini sudo wget podman catatonit \
  && apt-get autoremove -y && apt-get clean -y && rm -rf /var/lib/apt/lists/* \
  && wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.11/grpc_health_probe-linux-amd64 \
  && chmod +x /bin/grpc_health_probe
COPY --from=builder /build/bin/cordium-supervisor /usr/bin/
COPY --from=builder /build/bin/cordium-workspace /usr/bin/
COPY --from=builder /build/bin/cordium-git-cred-helper /usr/bin/
COPY --from=builder /build/bin/octelium /usr/bin/
COPY --from=builder /build/bin/octeliumctl /usr/bin/

ENTRYPOINT ["tini", "--"]
CMD ["/usr/bin/cordium-supervisor"]

