FROM golang:1.26 AS build
WORKDIR /src
COPY app/go.mod app/go.sum ./app/
WORKDIR /src/app
RUN go mod download
COPY app/ ./
RUN CGO_ENABLED=0 go build -o /out/app .

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/app /app
EXPOSE 9100
ENTRYPOINT ["/app"]
