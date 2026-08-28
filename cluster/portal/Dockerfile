FROM node as builder-web
COPY cluster/portal/portal/web/package /build
RUN cd /build && npm install && npm run build

FROM golang:1.26.6 as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
COPY --from=builder-web /build/dist /build/cluster/portal/portal/web
RUN make build-portal
FROM alpine
RUN apk --no-cache add ca-certificates
RUN adduser -S -D -H -u 3454 -h /app octelium
USER octelium
COPY --from=builder /build/bin/cordium-portal /app/
ENTRYPOINT ["/app/cordium-portal"]
