FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/dns .

FROM gcr.io/distroless/static
COPY --from=build /out/dns /dns
EXPOSE 5353/udp
ENTRYPOINT ["/dns"]
