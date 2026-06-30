use spin_sdk::http::{IntoResponse, Request, Response};
use spin_sdk::http_component;

#[http_component]
fn handle(_req: Request) -> anyhow::Result<impl IntoResponse> {
    let body = serde_json::json!({ "version": "v1-wasm", "runtime": "spin" });
    Ok(Response::builder()
        .status(200)
        .header("content-type", "application/json")
        .body(serde_json::to_vec(&body)?)
        .build())
}
