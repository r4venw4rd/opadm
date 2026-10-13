# Build: CGO'suz (modernc.org/sqlite), statik binary
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /opadm .
# /data burada hazırlanır: run stage'de shell yok (distroless).
# 65532 = distroless nonroot kullanıcısı.
RUN mkdir -p /data && chown 65532:65532 /data

# Run: non-root, volume'da SQLite
FROM gcr.io/distroless/static-debian12:nonroot
ENV PORT=8080 DB_PATH=/data/requests.db
VOLUME /data
EXPOSE 8080
COPY --from=build /opadm /opadm
COPY --from=build --chown=65532:65532 /data /data
USER nonroot:nonroot
ENTRYPOINT ["/opadm"]
