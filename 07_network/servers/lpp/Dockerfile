FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/lpp .

FROM gcr.io/distroless/static
COPY --from=build /out/lpp /lpp
EXPOSE 9004
ENTRYPOINT ["/lpp"]
