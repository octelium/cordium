
FROM golang:1.26.4 as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN make build-cli-cordium

FROM alpine:3.23
RUN apk --no-cache add ca-certificates
ENV OCTELIUM_CONTAINER_MODE=true
COPY --from=builder /build/bin/cordium /usr/bin/
ENTRYPOINT ["/usr/bin/cordium"]