use axum::{extract::State, routing::get, Json, Router};
use chrono::{SecondsFormat, Utc};
use serde_json::{json, Value};
use std::env;
use std::time::Instant;

fn fibonacci(n: u32) -> u64 {
    if n <= 1 {
        return n as u64;
    }
    fibonacci(n - 1) + fibonacci(n - 2)
}

fn get_thread_id() -> String {
    let debug_str = format!("{:?}", std::thread::current().id());
    let id_num = debug_str
        .trim_start_matches("ThreadId(")
        .trim_end_matches(')')
        .to_string();
    format!("tokio-worker-{}", id_num)
}

#[derive(Clone)]
struct AppState {
    heavy_calc_n: u32,
}

async fn health() -> Json<Value> {
    Json(json!({
        "status": "ok",
        "language": "rust-axum"
    }))
}

async fn heavy(State(state): State<AppState>) -> Json<Value> {
    let started_at = Utc::now();
    let start = Instant::now();

    fibonacci(state.heavy_calc_n);

    let duration_ms = start.elapsed().as_millis() as u64;
    let finished_at = Utc::now();

    Json(json!({
        "language": "rust-axum",
        "threadId": get_thread_id(),
        "startedAt": started_at.to_rfc3339_opts(SecondsFormat::Millis, true),
        "finishedAt": finished_at.to_rfc3339_opts(SecondsFormat::Millis, true),
        "durationMs": duration_ms
    }))
}

fn main() {
    let heavy_calc_n: u32 = env::var("HEAVY_CALC_N")
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(43);

    let workers: usize = env::var("TOKIO_WORKERS")
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(4);

    let runtime = tokio::runtime::Builder::new_multi_thread()
        .worker_threads(workers)
        .enable_all()
        .build()
        .expect("Failed to build Tokio runtime");

    runtime.block_on(async {
        let app = Router::new()
            .route("/health", get(health))
            .route("/heavy", get(heavy))
            .with_state(AppState { heavy_calc_n });

        let listener = tokio::net::TcpListener::bind("0.0.0.0:8080")
            .await
            .expect("Failed to bind");

        println!(
            "Rust (Axum) server listening on :8080 (workers={})",
            workers
        );

        axum::serve(listener, app).await.expect("Server error");
    });
}
