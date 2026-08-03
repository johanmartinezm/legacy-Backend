FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the pre-compiled Linux binary
COPY server_linux /app/server
COPY config.docker.yaml /app/config.yaml

# Copy Firebase credentials (optional, using wildcard to prevent error if missing)
COPY firebase-service-account.json* /app/

# Copy Google Mailer credentials
COPY google-mailer-service-account.json /app/

EXPOSE 8080

CMD ["./server"]
