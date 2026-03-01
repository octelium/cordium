
FROM golang:1.25.7 as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN make build-mockapiserver
FROM alpine
RUN apk --no-cache add ca-certificates
RUN adduser -S -D -H -u 3454 -h /app octelium
RUN wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.11/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe
USER octelium
COPY --from=builder /build/bin/cordium-mockapiserver /app/
EXPOSE 8080
ENTRYPOINT ["/app/cordium-mockapiserver"]

