# Runtime image. Build the linux binary first: `make docker-bin`
FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY server/bin/workbench /app/workbench
COPY web /app/web
RUN chmod +x /app/workbench
EXPOSE 18527
ENTRYPOINT ["/app/workbench"]
