workers ENV.fetch("WEB_CONCURRENCY", 4)
threads_count = ENV.fetch("RAILS_MAX_THREADS", 1).to_i
threads threads_count, threads_count

port ENV.fetch("PORT", 8080)

environment ENV.fetch("RAILS_ENV", "production")

preload_app!
