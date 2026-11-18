FROM golang:1.27-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /server

FROM scratch
# CA certificates, so the database connection can verify the server (sslmode=verify-full)
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /server /server
ENV GIN_MODE=release
# run as an unprivileged user (nobody); the service needs no file system access
USER 65534:65534
EXPOSE 8080
CMD ["/server"]
