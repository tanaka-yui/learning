FROM ruby:3.4 AS builder

WORKDIR /app

RUN gem install rails --no-document && \
    rails new . --api --minimal --skip-git --skip-test --skip-system-test --skip-active-record --skip-action-mailer --skip-action-mailbox --skip-action-text --skip-active-job --skip-active-storage --skip-action-cable --no-interaction

COPY config/routes.rb /app/config/routes.rb
COPY config/puma.rb /app/config/puma.rb
COPY app/controllers/heavy_controller.rb /app/app/controllers/heavy_controller.rb

FROM ruby:3.4-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    libgmp-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app /app
COPY --from=builder /usr/local/bundle /usr/local/bundle

ENV RAILS_ENV=production
ENV RAILS_LOG_TO_STDOUT=true
ENV SECRET_KEY_BASE=dummy-secret-for-demo-only

EXPOSE 8080

CMD ["bundle", "exec", "puma", "-C", "config/puma.rb"]
