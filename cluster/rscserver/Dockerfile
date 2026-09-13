
FROM golang:1.26.8 as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN make build-rscserver
FROM alpine:3.24
RUN apk --no-cache add ca-certificates
RUN adduser -S -D -H -u 3454 -h /app octelium
USER octelium
EXPOSE 8080
COPY --from=builder /build/bin/cordium-rscserver /app/
ENTRYPOINT ["/app/cordium-rscserver"]