# Used by goreleaser
FROM alpine:latest AS certs
RUN apk --no-cache add ca-certificates

FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/coro-proxy-agent"]
COPY coro-proxy-agent /
