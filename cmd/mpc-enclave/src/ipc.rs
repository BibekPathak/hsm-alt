use crate::crypto::{EpochState, KeyShare};
use crate::enclave::{Enclave, EnclaveState};
use std::collections::HashMap;
use std::sync::Arc;
use std::sync::RwLock;
use tracing::{error, info};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InitRequest {
    pub node_id: u32,
    pub cluster_id: String,
    pub threshold: u32,
    pub total_shares: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InitResponse {
    pub success: bool,
    pub error: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StatusRequest {}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StatusResponse {
    pub state: String,
    pub epoch: u32,
    pub public_key: Vec<u8>,
    pub initialized: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DKGRequest {}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DKGResponse {
    pub success: bool,
    pub error: String,
    pub round1_message: Vec<u8>,
    pub share: Vec<u8>,
    pub commitments: Vec<Vec<u8>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PublicKeyRequest {}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PublicKeyResponse {
    pub public_key: Vec<u8>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SignRequest {
    pub message: Vec<u8>,
    pub signer_ids: Vec<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SignResponse {
    pub success: bool,
    pub error: String,
    pub partial_signature: Vec<u8>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResharingRequest {}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResharingResponse {
    pub success: bool,
    pub error: String,
    pub new_share: Vec<u8>,
    pub new_commitments: Vec<Vec<u8>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EpochRequest {}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EpochResponse {
    pub success: bool,
    pub new_epoch: u32,
    pub new_public_key: Vec<u8>,
    pub new_commitment: Vec<u8>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AttestRequest {}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AttestResponse {
    pub quote: Vec<u8>,
    pub measurement: Vec<u8>,
}

#[derive(Clone)]
pub struct EnclaveService {
    enclave: Arc<RwLock<Enclave>>,
}

impl EnclaveService {
    pub fn new(node_id: u32, threshold: u32, total_shares: u32) -> Self {
        let enclave = Enclave::new(node_id, threshold, total_shares);
        Self {
            enclave: Arc::new(RwLock::new(enclave)),
        }
    }

    pub fn initialize(&self, req: InitRequest) -> InitResponse {
        info!("Initializing enclave for node {} in cluster {}", req.node_id, req.cluster_id);

        let enclave = self.enclave.read().unwrap();

        match enclave.initialize(req.cluster_id) {
            Ok(()) => InitResponse {
                success: true,
                error: String::new(),
            },
            Err(e) => {
                error!("Failed to initialize enclave: {}", e);
                InitResponse {
                    success: false,
                    error: e.to_string(),
                }
            }
        }
    }

    pub fn get_status(&self) -> StatusResponse {
        let enclave = self.enclave.read().unwrap();

        let state = match enclave.get_state() {
            EnclaveState::Uninitialized => "uninitialized".to_string(),
            EnclaveState::Initializing => "initializing".to_string(),
            EnclaveState::Ready => "ready".to_string(),
            EnclaveState::KeyGeneration => "key_generation".to_string(),
            EnclaveState::Signing => "signing".to_string(),
            EnclaveState::Resharing => "resharing".to_string(),
            EnclaveState::Error(msg) => format!("error:{}", msg),
        };

        let public_key = enclave.get_public_key().ok();

        StatusResponse {
            state,
            epoch: enclave.get_current_epoch(),
            public_key: public_key.unwrap_or_default(),
            initialized: enclave.is_initialized(),
        }
    }

    pub fn start_dkg(&self) -> DKGResponse {
        let enclave = self.enclave.read().unwrap();

        match enclave.start_dkg(vec![]) {
            Ok((round1_msg, comms)) => DKGResponse {
                success: true,
                error: String::new(),
                round1_message: round1_msg,
                share: vec![],
                commitments: comms,
            },
            Err(e) => {
                error!("DKG failed: {}", e);
                DKGResponse {
                    success: false,
                    error: e.to_string(),
                    round1_message: vec![],
                    share: vec![],
                    commitments: vec![],
                }
            }
        }
    }

    pub fn get_public_key(&self) -> Result<PublicKeyResponse, String> {
        let enclave = self.enclave.read().unwrap();

        match enclave.get_public_key() {
            Ok(pk) => Ok(PublicKeyResponse { public_key: pk }),
            Err(e) => Err(e.to_string()),
        }
    }

    pub fn sign(&self, req: SignRequest) -> SignResponse {
        let enclave = self.enclave.read().unwrap();

        match enclave.sign(req.message, req.signer_ids) {
            Ok(partial_sig) => SignResponse {
                success: true,
                error: String::new(),
                partial_signature: partial_sig,
            },
            Err(e) => {
                error!("Signing failed: {}", e);
                SignResponse {
                    success: false,
                    error: e.to_string(),
                    partial_signature: vec![],
                }
            }
        }
    }

    pub fn start_resharing(&self, _req: ResharingRequest) -> ResharingResponse {
        ResharingResponse {
            success: true,
            error: String::new(),
            new_share: vec![],
            new_commitments: vec![],
        }
    }

    pub fn evolve_key(&self) -> EpochResponse {
        let enclave = self.enclave.read().unwrap();

        match enclave.evolve_key() {
            Ok(epoch_state) => EpochResponse {
                success: true,
                new_epoch: epoch_state.epoch,
                new_public_key: epoch_state.public_key,
                new_commitment: epoch_state.commitment,
            },
            Err(e) => {
                error!("Key evolution failed: {}", e);
                EpochResponse {
                    success: false,
                    new_epoch: 0,
                    new_public_key: vec![],
                    new_commitment: vec![],
                }
            }
        }
    }

    pub fn get_attestation(&self) -> AttestResponse {
        AttestResponse {
            quote: vec![],
            measurement: vec![],
        }
    }
}

pub async fn run_enclave_server(
    addr: std::net::SocketAddr,
    node_id: u32,
    threshold: u32,
    total_shares: u32,
) -> Result<(), Box<dyn std::error::Error>> {
    use tokio::io::{AsyncReadExt, AsyncWriteExt};

    let service = EnclaveService::new(node_id, threshold, total_shares);

    let listener = tokio::net::TcpListener::bind(addr).await?;
    info!("Enclave server listening on {}", addr);

    loop {
        let (mut socket, _) = listener.accept().await?;
        let service = service.clone();

        tokio::spawn(async move {
            let mut buf = [0u8; 4096];
            
            loop {
                match socket.read(&mut buf).await {
                    Ok(0) => break,
                    Ok(n) => {
                        let response = process_request(&buf[..n]);
                        let _ = socket.write_all(&response).await;
                    }
                    Err(_) => break,
                }
            }
        });
    }
}

fn process_request(data: &[u8]) -> Vec<u8> {
    serde_json::json!({"status": "ok"}).to_string().into_bytes()
}
