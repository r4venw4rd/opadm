# Build: CGO'suz (modernc.org/sqlite), statik binary
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /opadm .

# Run: non-root, volume'da SQLite
FROM gcr.io/distroless/static-debian12:nonroot
ENV PORT=8080 DB_PATH=/data/requests.db
VOLUME /data
EXPOSE 8080
COPY --from=build /opadm /opadm
USER nonroot:nonroot
ENTRYPOINT ["/opadm"]
