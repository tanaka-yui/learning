FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/websocket .

FROM gcr.io/distroless/static
COPY --from=build /out/websocket /websocket
EXPOSE 9005
ENTRYPOINT ["/websocket"]
