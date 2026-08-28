
FROM golang:1.26.6 as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN make build-mockapiserver
FROM alpine
RUN apk --no-cache add ca-certificates
RUN adduser -S -D -H -u 3454 -h /app octelium
USER octelium
COPY --from=builder /build/bin/cordium-mockapiserver /app/
EXPOSE 8080
ENTRYPOINT ["/app/cordium-mockapiserver"]

