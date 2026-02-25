FROM gradle:8-jdk21 AS builder

WORKDIR /app

COPY build.gradle settings.gradle ./
COPY src/ ./src/

RUN gradle bootJar --no-daemon

FROM eclipse-temurin:21-jre

COPY --from=builder /app/build/libs/*.jar /app/app.jar

EXPOSE 8081

ENTRYPOINT ["java", "-jar", "/app/app.jar"]
