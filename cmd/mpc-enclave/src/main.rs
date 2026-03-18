use std::env;
use std::net::SocketAddr;
use tracing_subscriber::fmt::format::FmtSpan;
use mpc_enclave::ipc::run_enclave_server;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt()
        .with_span_events(FmtSpan::CLOSE)
        .with_target(true)
        .with_level(true)
        .init();

    let node_id: u32 = env::var("NODE_ID")
        .unwrap_or_else(|_| "1".to_string())
        .parse()
        .unwrap_or(1);

    let threshold: u32 = env::var("THRESHOLD")
        .unwrap_or_else(|_| "3".to_string())
        .parse()
        .unwrap_or(3);

    let total_shares: u32 = env::var("TOTAL_SHARES")
        .unwrap_or_else(|_| "5".to_string())
        .parse()
        .unwrap_or(5);

    let port: u16 = env::var("ENCLAVE_PORT")
        .unwrap_or_else(|_| "7002".to_string())
        .parse()
        .unwrap_or(7002);

    let addr = SocketAddr::from(([127, 0, 0, 1], port));

    tracing::info!(
        "Starting MPC Enclave - Node ID: {}, Threshold: {}/{}",
        node_id,
        threshold,
        total_shares
    );

    run_enclave_server(addr, node_id, threshold, total_shares).await?;

    Ok(())
}
