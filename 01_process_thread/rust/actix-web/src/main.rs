use actix_web::{web, App, HttpResponse, HttpServer};
use chrono::{SecondsFormat, Utc};
use serde_json::json;
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
    format!("actix-worker-{}", id_num)
}

async fn health() -> HttpResponse {
    HttpResponse::Ok().json(json!({
        "status": "ok",
        "language": "rust"
    }))
}

async fn heavy(data: web::Data<AppState>) -> HttpResponse {
    let started_at = Utc::now();
    let start = Instant::now();

    fibonacci(data.heavy_calc_n);

    let duration_ms = start.elapsed().as_millis() as u64;
    let finished_at = Utc::now();

    HttpResponse::Ok().json(json!({
        "language": "rust",
        "threadId": get_thread_id(),
        "startedAt": started_at.to_rfc3339_opts(SecondsFormat::Millis, true),
        "finishedAt": finished_at.to_rfc3339_opts(SecondsFormat::Millis, true),
        "durationMs": duration_ms
    }))
}

struct AppState {
    heavy_calc_n: u32,
}

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    let heavy_calc_n: u32 = env::var("HEAVY_CALC_N")
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(43);

    let workers: usize = env::var("ACTIX_WORKERS")
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(4);

    println!("Rust server listening on :8080 (workers={})", workers);

    HttpServer::new(move || {
        App::new()
            .app_data(web::Data::new(AppState { heavy_calc_n }))
            .route("/health", web::get().to(health))
            .route("/heavy", web::get().to(heavy))
    })
    .workers(workers)
    .bind(("0.0.0.0", 8080))?
    .run()
    .await
}
