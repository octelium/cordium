
FROM golang:1.25.8 as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN make build-supervisor
RUN make build-workspace
RUN make build-workspace-git-cred-helper
RUN make build-cli-cordium


FROM alpine:3.23.3
RUN apk --no-cache add curl bash tini \
  shadow git ca-certificates podman buildah skopeo umoci \
  iptables ip6tables libcap runc sudo bash libc6-compat crun slirp4netns fuse-overlayfs \
  && mkdir -p /root/rootfs && wget -qO- https://dl-cdn.alpinelinux.org/alpine/v3.23/releases/x86_64/alpine-minirootfs-3.23.3-x86_64.tar.gz | tar xvz -C /root/rootfs \
  && curl -fsSL https://octelium.com/install.sh | bash \
  && cp /usr/local/bin/octelium /bin/ \
  && cp /usr/local/bin/octeliumctl /bin/ \
  && rm -rf octelium octeliumctl octops
COPY --from=builder /build/bin/cordium-supervisor /bin/
COPY --from=builder /build/bin/cordium-workspace /bin/
COPY --from=builder /build/bin/cordium-git-cred-helper /bin/
COPY --from=builder /build/bin/cordium /bin/

ENTRYPOINT ["tini", "--"]
CMD ["/bin/cordium-supervisor"]

