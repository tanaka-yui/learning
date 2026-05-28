FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/tcp-echo .

FROM gcr.io/distroless/static
COPY --from=build /out/tcp-echo /tcp-echo
EXPOSE 9001
ENTRYPOINT ["/tcp-echo"]
