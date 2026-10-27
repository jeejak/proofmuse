# Demo image: the server plus the synthetic eval set it serves as samples.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags=-s -o /out/app/proofmuse ./cmd/server \
 && mkdir -p /out/app/evalset /out/data \
 && cp -r evalset/records evalset/*.csv evalset/orders.json /out/app/evalset/ \
 && chmod -R a+rX /out/app \
 && chown 65532:65532 /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
COPY --from=build --chown=65532:65532 /out/data /data
ENV EVALSET=/app/evalset DATA_DIR=/data PORT=8080
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/app/proofmuse"]
