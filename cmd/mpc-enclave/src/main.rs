use std::env;
use mpc_enclave::ipc::Enclave;
use parking_lot::RwLock;
use std::sync::Arc;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt()
        .with_max_level(tracing::Level::INFO)
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

    tracing::info!(
        "Starting MPC Enclave - Node ID: {}, Threshold: {}/{}",
        node_id,
        threshold,
        total_shares
    );

    let enclave = Arc::new(RwLock::new(Enclave::new(node_id, threshold, total_shares)));

    let runtime = tokio::runtime::Runtime::new()?;
    runtime.block_on(async move {
        if let Err(e) = mpc_enclave::ipc::run_server(enclave, port).await {
            tracing::error!("Server error: {}", e);
        }
    });

    Ok(())
}