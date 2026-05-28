FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/udp-echo .

FROM gcr.io/distroless/static
COPY --from=build /out/udp-echo /udp-echo
EXPOSE 9002/udp
ENTRYPOINT ["/udp-echo"]
