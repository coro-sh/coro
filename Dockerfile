FROM busybox AS builder
RUN mkdir /apptemp

FROM scratch
COPY coro /
# Broker service will create a temp NATS resolver config file in /tmp on startup
COPY --from=builder /apptemp /tmp
ENTRYPOINT ["/coro"]
