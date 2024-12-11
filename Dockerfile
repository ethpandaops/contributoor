FROM alpine:latest
COPY sentry /usr/local/bin/
RUN mkdir -p /config
ENTRYPOINT ["sentry"] 