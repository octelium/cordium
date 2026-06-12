FROM golang:1.26.4 as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN make build-genesis
FROM alpine
RUN apk --no-cache add ca-certificates bash curl openssl
RUN curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash
# RUN apk del bash curl openssl
RUN adduser -S -D -h /app octelium
# RUN chown octelium /app
USER octelium
COPY --from=builder /build/bin/cordium-genesis /app/
ENTRYPOINT ["/app/cordium-genesis"]
