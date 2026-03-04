class HeavyController < ApplicationController
  HEAVY_CALC_N = ENV.fetch("HEAVY_CALC_N", "40").to_i

  def health
    render json: { status: "ok", language: "ruby" }
  end

  def heavy
    n = params[:n].present? ? params[:n].to_i : HEAVY_CALC_N
    started_at = Time.now.utc.strftime("%Y-%m-%dT%H:%M:%S.%3NZ")
    start_ns = Process.clock_gettime(Process::CLOCK_MONOTONIC, :nanosecond)

    fibonacci(n)

    end_ns = Process.clock_gettime(Process::CLOCK_MONOTONIC, :nanosecond)
    finished_at = Time.now.utc.strftime("%Y-%m-%dT%H:%M:%S.%3NZ")

    duration_ms = ((end_ns - start_ns) / 1_000_000.0).round

    render json: {
      language: "ruby",
      threadId: "puma-worker-#{Process.pid}",
      startedAt: started_at,
      finishedAt: finished_at,
      durationMs: duration_ms
    }
  end

  private

  def fibonacci(n)
    return n if n <= 1
    fibonacci(n - 1) + fibonacci(n - 2)
  end
end
