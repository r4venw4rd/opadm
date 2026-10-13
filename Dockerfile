# Build: CGO'suz, statik binary (+ Supabase TLS için CA sertifikası)
FROM golang:1.26-alpine AS build
RUN apk add --no-cache ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /opadm .

# Run: non-root, DB Supabase'te (dosya yok, volume yok)
FROM gcr.io/distroless/static-debian12:nonroot
ENV PORT=8080
EXPOSE 8080
COPY --from=build /opadm /opadm
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER nonroot:nonroot
ENTRYPOINT ["/opadm"]
