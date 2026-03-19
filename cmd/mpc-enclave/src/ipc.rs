//! HTTP Server Implementation using Axum
//! Handles communication between Go Node and Rust Enclave via HTTP/JSON

use crate::crypto::{self, DKGOutput, EpochState, KeyShare, PartialSignature, SignRound1Output};
use axum::{
    extract::State,
    http::StatusCode,
    routing::{get, post},
    Json, Router,
};
use parking_lot::RwLock;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::Arc;
use tower_http::cors::{Any, CorsLayer};

#[derive(Debug, Clone, PartialEq)]
pub enum EnclaveState {
    Uninitialized,
    Initializing,
    Ready,
    KeyGeneration,
    Signing,
    Resharing,
    Error(String),
}

pub struct Enclave {
    node_id: u32,
    cluster_id: RwLock<Option<String>>,
    threshold: u32,
    total_shares: u32,
    state: RwLock<EnclaveState>,

    dkg_output: RwLock<Option<DKGOutput>>,
    my_share: RwLock<Option<KeyShare>>,
    my_nonces: RwLock<Option<Vec<u8>>>,
    current_epoch: RwLock<u32>,
    epoch_state: RwLock<Option<EpochState>>,

    signing_message: RwLock<Option<Vec<u8>>>,
    signing_commitments: RwLock<HashMap<u32, Vec<u8>>>,
}

impl Enclave {
    pub fn new(node_id: u32, threshold: u32, total_shares: u32) -> Self {
        Self {
            node_id,
            cluster_id: RwLock::new(None),
            threshold,
            total_shares,
            state: RwLock::new(EnclaveState::Uninitialized),
            dkg_output: RwLock::new(None),
            my_share: RwLock::new(None),
            my_nonces: RwLock::new(None),
            current_epoch: RwLock::new(0),
            epoch_state: RwLock::new(None),
            signing_message: RwLock::new(None),
            signing_commitments: RwLock::new(HashMap::new()),
        }
    }

    pub fn initialize(&self, cluster_id: String) -> Result<(), String> {
        *self.cluster_id.write() = Some(cluster_id);
        *self.state.write() = EnclaveState::Ready;
        Ok(())
    }

    pub fn get_status(&self) -> (String, u32, Vec<u8>, bool) {
        let state = match &*self.state.read() {
            EnclaveState::Uninitialized => "uninitialized".to_string(),
            EnclaveState::Initializing => "initializing".to_string(),
            EnclaveState::Ready => "ready".to_string(),
            EnclaveState::KeyGeneration => "key_generation".to_string(),
            EnclaveState::Signing => "signing".to_string(),
            EnclaveState::Resharing => "resharing".to_string(),
            EnclaveState::Error(msg) => format!("error:{}", msg),
        };

        let epoch = *self.current_epoch.read();
        let initialized = matches!(*self.state.read(), EnclaveState::Ready);

        let public_key = self
            .dkg_output
            .read()
            .as_ref()
            .map(|o| o.public_key.key.clone())
            .unwrap_or_default();

        (state, epoch, public_key, initialized)
    }

    pub fn dkg_start(&self, _min_signers: u16, _max_signers: u16) -> Result<DkgStartResult, String> {
        if !matches!(*self.state.read(), EnclaveState::Ready) {
            return Err("Node not ready for DKG".to_string());
        }

        *self.state.write() = EnclaveState::KeyGeneration;

        let output = crypto::dkg_generate(self.threshold as u16, self.total_shares as u16)
            .map_err(|e| e.to_string())?;

        let our_share = output
            .shares
            .iter()
            .find(|s| s.index == self.node_id)
            .cloned()
            .ok_or_else(|| "Our share not found".to_string())?;

        *self.dkg_output.write() = Some(output.clone());
        *self.my_share.write() = Some(our_share);
        *self.current_epoch.write() = 1;

        *self.epoch_state.write() = Some(EpochState::new(output.public_key.key.clone()));

        *self.state.write() = EnclaveState::Ready;

        Ok(DkgStartResult {
            success: true,
            round: 1,
            round1_data: vec![],
        })
    }

    pub fn get_public_key(&self) -> Result<Vec<u8>, String> {
        let output = self.dkg_output.read();
        match output.as_ref() {
            Some(o) => Ok(o.public_key.key.clone()),
            None => Err("Public key not available".to_string()),
        }
    }

    pub fn get_key_share(&self) -> Result<(Vec<u8>, u32), String> {
        let share = self.my_share.read();
        match share.as_ref() {
            Some(s) => Ok((s.key_package.clone(), s.index)),
            None => Err("Key share not available".to_string()),
        }
    }

    pub fn sign_round1(&self) -> Result<SignRound1Result, String> {
        let share = self.my_share.read();
        let key_package = share.as_ref().ok_or("Key share not available")?;

        let output = crypto::sign_round1(&key_package.key_package).map_err(|e| e.to_string())?;

        *self.my_nonces.write() = Some(output.nonces.clone());

        Ok(SignRound1Result {
            success: true,
            nonce_commitment: output.nonces,
            commitment: output.commitment,
        })
    }

    pub fn add_commitment(&self, from_node: u32, commitment: Vec<u8>) {
        self.signing_commitments
            .write()
            .insert(from_node, commitment);
    }

    pub fn sign_round2(&self, signing_package_bytes: Vec<u8>) -> Result<SignRound2Result, String> {
        let share = self.my_share.read();
        let key_package = share.as_ref().ok_or("Key share not available")?;

        let nonces = self.my_nonces.read();
        let nonces_bytes = nonces.as_ref().ok_or("Nonces not available")?;

        let commitments = self.signing_commitments.read();
        let all_commitments: std::collections::HashMap<u32, Vec<u8>> = commitments.clone();

        let partial = crypto::sign_round2(
            &signing_package_bytes,
            nonces_bytes,
            &key_package.key_package,
            &all_commitments,
        )
        .map_err(|e| e.to_string())?;

        *self.state.write() = EnclaveState::Ready;

        Ok(SignRound2Result {
            success: true,
            partial_signature: partial.signature_share,
            commitment: partial.commitment,
        })
    }
}

#[derive(Clone)]
struct AppState {
    enclave: Arc<RwLock<Enclave>>,
}

#[derive(Debug, Serialize, Deserialize)]
struct InitRequest {
    cluster_id: String,
}

#[derive(Debug, Serialize, Deserialize)]
struct InitResponse {
    success: bool,
    error: String,
}

#[derive(Debug, Serialize, Deserialize)]
struct StatusResponse {
    state: String,
    epoch: u32,
    public_key: Vec<u8>,
    initialized: bool,
}

#[derive(Debug, Serialize, Deserialize)]
struct DkgStartRequest {
    min_signers: u32,
    max_signers: u32,
}

#[derive(Debug, Serialize, Deserialize)]
struct DkgStartResult {
    success: bool,
    round: u32,
    round1_data: Vec<u8>,
}

#[derive(Debug, Serialize, Deserialize)]
struct SignRound1Result {
    success: bool,
    nonce_commitment: Vec<u8>,
    commitment: Vec<u8>,
}

#[derive(Debug, Serialize, Deserialize)]
struct SignRound2Request {
    signing_package: Vec<u8>,
}

#[derive(Debug, Serialize, Deserialize)]
struct SignRound2Result {
    success: bool,
    partial_signature: Vec<u8>,
    commitment: Vec<u8>,
}

#[derive(Debug, Serialize, Deserialize)]
struct PublicKeyResponse {
    public_key: Vec<u8>,
}

#[derive(Debug, Serialize, Deserialize)]
struct KeyShareResponse {
    key_share: Vec<u8>,
    index: u32,
}

async fn initialize(
    State(state): State<AppState>,
    Json(req): Json<InitRequest>,
) -> Json<InitResponse> {
    let enclave = state.enclave.read();
    match enclave.initialize(req.cluster_id) {
        Ok(()) => Json(InitResponse {
            success: true,
            error: String::new(),
        }),
        Err(e) => Json(InitResponse {
            success: false,
            error: e,
        }),
    }
}

async fn get_status(State(state): State<AppState>) -> Json<StatusResponse> {
    let enclave = state.enclave.read();
    let (state_str, epoch, public_key, initialized) = enclave.get_status();
    Json(StatusResponse {
        state: state_str,
        epoch,
        public_key,
        initialized,
    })
}

async fn dkg_start(
    State(state): State<AppState>,
    Json(req): Json<DkgStartRequest>,
) -> Json<DkgStartResult> {
    let enclave = state.enclave.read();
    match enclave.dkg_start(req.min_signers as u16, req.max_signers as u16) {
        Ok(result) => Json(result),
        Err(e) => Json(DkgStartResult {
            success: false,
            round: 0,
            round1_data: e.as_bytes().to_vec(),
        }),
    }
}

async fn sign_round1(State(state): State<AppState>) -> Json<SignRound1Result> {
    let enclave = state.enclave.read();
    match enclave.sign_round1() {
        Ok(result) => Json(result),
        Err(e) => Json(SignRound1Result {
            success: false,
            nonce_commitment: vec![],
            commitment: e.as_bytes().to_vec(),
        }),
    }
}

async fn sign_round2(
    State(state): State<AppState>,
    Json(req): Json<SignRound2Request>,
) -> Json<SignRound2Result> {
    let enclave = state.enclave.read();
    match enclave.sign_round2(req.signing_package) {
        Ok(result) => Json(result),
        Err(e) => Json(SignRound2Result {
            success: false,
            partial_signature: vec![],
            commitment: e.as_bytes().to_vec(),
        }),
    }
}

async fn get_public_key(State(state): State<AppState>) -> Json<PublicKeyResponse> {
    let enclave = state.enclave.read();
    match enclave.get_public_key() {
        Ok(pk) => Json(PublicKeyResponse { public_key: pk }),
        Err(_) => Json(PublicKeyResponse {
            public_key: vec![],
        }),
    }
}

async fn get_key_share(State(state): State<AppState>) -> Json<KeyShareResponse> {
    let enclave = state.enclave.read();
    match enclave.get_key_share() {
        Ok((share, idx)) => Json(KeyShareResponse {
            key_share: share,
            index: idx,
        }),
        Err(_) => Json(KeyShareResponse {
            key_share: vec![],
            index: 0,
        }),
    }
}

pub async fn run_server(enclave: Arc<RwLock<Enclave>>, port: u16) -> Result<(), Box<dyn std::error::Error>> {
    let state = AppState { enclave };

    let cors = CorsLayer::new()
        .allow_origin(Any)
        .allow_methods(Any)
        .allow_headers(Any);

    let app = Router::new()
        .route("/initialize", post(initialize))
        .route("/status", get(get_status))
        .route("/dkg/start", post(dkg_start))
        .route("/sign/round1", post(sign_round1))
        .route("/sign/round2", post(sign_round2))
        .route("/public-key", get(get_public_key))
        .route("/key-share", get(get_key_share))
        .layer(cors)
        .with_state(state);

    let addr = SocketAddr::from(([0, 0, 0, 0], port));
    tracing::info!("Enclave HTTP server listening on {}", addr);

    let listener = tokio::net::TcpListener::bind(addr).await?;
    axum::serve(listener, app).await?;

    Ok(())
}
