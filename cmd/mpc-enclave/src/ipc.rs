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
use std::collections::{BTreeMap, HashMap};
use std::net::SocketAddr;
use std::sync::Arc;
use tower_http::cors::{Any, CorsLayer};

#[allow(unused_imports)]
use frost_ed25519 as frost;

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
    
    // DKG state
    dkg_secret_pkg1: RwLock<Option<Vec<u8>>>,
    dkg_secret_pkg2: RwLock<Option<Vec<u8>>>,
    
    my_nonces: RwLock<Option<Vec<u8>>>,
    current_epoch: RwLock<u32>,
    epoch_state: RwLock<Option<EpochState>>,

    signing_message: RwLock<Option<Vec<u8>>>,
    signing_commitments: RwLock<HashMap<u32, Vec<u8>>>,
    
    // Full key packages for signing
    key_package: RwLock<Option<Vec<u8>>>,
    pubkey_package: RwLock<Option<Vec<u8>>>,
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
            dkg_secret_pkg1: RwLock::new(None),
            dkg_secret_pkg2: RwLock::new(None),
            my_nonces: RwLock::new(None),
            current_epoch: RwLock::new(0),
            epoch_state: RwLock::new(None),
            signing_message: RwLock::new(None),
            signing_commitments: RwLock::new(HashMap::new()),
            key_package: RwLock::new(None),
            pubkey_package: RwLock::new(None),
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

    pub fn dkg_part1(&self, min_signers: u16, max_signers: u16) -> Result<DKGPart1Result, String> {
        if !matches!(*self.state.read(), EnclaveState::Ready) && !matches!(*self.state.read(), EnclaveState::Uninitialized) {
            return Err("Node not ready for DKG".to_string());
        }

        *self.state.write() = EnclaveState::KeyGeneration;

        let output = crypto::dkg_part1(self.node_id, max_signers, min_signers)
            .map_err(|e| e.to_string())?;

        *self.dkg_secret_pkg1.write() = Some(output.secret_package.clone());

        Ok(DKGPart1Result {
            success: true,
            error: String::new(),
            secret_package: output.secret_package,
            round1_package: output.package,
        })
    }

    pub fn dkg_part2(&self, round1_packages: std::collections::BTreeMap<u32, Vec<u8>>) -> Result<DKGPart2Result, String> {
        let secret_pkg = self.dkg_secret_pkg1.read();
        let secret_bytes = secret_pkg.as_ref().ok_or("No secret package from round 1")?;

        let output = crypto::dkg_part2(secret_bytes, &round1_packages)
            .map_err(|e| e.to_string())?;

        *self.dkg_secret_pkg2.write() = Some(output.secret_package.clone());

        Ok(DKGPart2Result {
            success: true,
            error: String::new(),
            secret_package: output.secret_package,
            round2_packages: output.packages,
        })
    }

    pub fn dkg_part3(
        &self,
        round1_packages: std::collections::BTreeMap<u32, Vec<u8>>,
        round2_packages: std::collections::BTreeMap<u32, Vec<u8>>,
    ) -> Result<DKGPart3Result, String> {
        let secret_pkg = self.dkg_secret_pkg2.read();
        let secret_bytes = secret_pkg.as_ref().ok_or("No secret package from round 2")?;

        let output = crypto::dkg_part3(secret_bytes, &round1_packages, &round2_packages)
            .map_err(|e| e.to_string())?;

        let key_package = serde_json::from_slice::<frost::keys::KeyPackage>(&output.key_package)
            .map_err(|e| e.to_string())?;
        
        let pubkey_package = serde_json::from_slice::<frost::keys::PublicKeyPackage>(&output.pubkey_package)
            .map_err(|e| e.to_string())?;

        *self.key_package.write() = Some(output.key_package.clone());
        *self.pubkey_package.write() = Some(output.pubkey_package.clone());
        
        *self.my_share.write() = Some(KeyShare {
            index: self.node_id,
            key_package: output.key_package.clone(),
        });
        
        *self.dkg_output.write() = Some(DKGOutput {
            shares: vec![],
            public_key: crypto::PublicKey {
                key: pubkey_package.verifying_key().serialize().unwrap_or_default(),
            },
            public_key_package: output.pubkey_package.clone(),
        });
        
        *self.current_epoch.write() = 1;
        *self.state.write() = EnclaveState::Ready;

        Ok(DKGPart3Result {
            success: true,
            error: String::new(),
            key_package: output.key_package,
            pubkey_package: output.pubkey_package,
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
struct DKGPart1Request {
    min_signers: u32,
    max_signers: u32,
}

#[derive(Debug, Serialize, Deserialize)]
struct DKGPart1Result {
    success: bool,
    error: String,
    secret_package: Vec<u8>,
    round1_package: Vec<u8>,
}

#[derive(Debug, Serialize, Deserialize)]
struct DKGPart2Request {
    round1_packages: std::collections::BTreeMap<u32, Vec<u8>>,
}

#[derive(Debug, Serialize, Deserialize)]
struct DKGPart2Result {
    success: bool,
    error: String,
    secret_package: Vec<u8>,
    round2_packages: std::collections::BTreeMap<u32, Vec<u8>>,
}

#[derive(Debug, Serialize, Deserialize)]
struct DKGPart3Request {
    round1_packages: std::collections::BTreeMap<u32, Vec<u8>>,
    round2_packages: std::collections::BTreeMap<u32, Vec<u8>>,
}

#[derive(Debug, Serialize, Deserialize)]
struct DKGPart3Result {
    success: bool,
    error: String,
    key_package: Vec<u8>,
    pubkey_package: Vec<u8>,
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

async fn dkg_part1(
    State(state): State<AppState>,
    Json(req): Json<DKGPart1Request>,
) -> Json<DKGPart1Result> {
    let enclave = state.enclave.read();
    match enclave.dkg_part1(req.min_signers as u16, req.max_signers as u16) {
        Ok(result) => Json(DKGPart1Result {
            success: true,
            error: String::new(),
            secret_package: result.secret_package,
            round1_package: result.round1_package,
        }),
        Err(e) => Json(DKGPart1Result {
            success: false,
            error: e,
            secret_package: vec![],
            round1_package: vec![],
        }),
    }
}

async fn dkg_part2(
    State(state): State<AppState>,
    Json(req): Json<DKGPart2Request>,
) -> Json<DKGPart2Result> {
    let enclave = state.enclave.read();
    match enclave.dkg_part2(req.round1_packages) {
        Ok(result) => Json(DKGPart2Result {
            success: true,
            error: String::new(),
            secret_package: result.secret_package,
            round2_packages: result.round2_packages,
        }),
        Err(e) => Json(DKGPart2Result {
            success: false,
            error: e,
            secret_package: vec![],
            round2_packages: std::collections::BTreeMap::new(),
        }),
    }
}

async fn dkg_part3(
    State(state): State<AppState>,
    Json(req): Json<DKGPart3Request>,
) -> Json<DKGPart3Result> {
    let enclave = state.enclave.read();
    match enclave.dkg_part3(req.round1_packages, req.round2_packages) {
        Ok(result) => Json(DKGPart3Result {
            success: true,
            error: String::new(),
            key_package: result.key_package,
            pubkey_package: result.pubkey_package,
        }),
        Err(e) => Json(DKGPart3Result {
            success: false,
            error: e,
            key_package: vec![],
            pubkey_package: vec![],
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
        .route("/dkg/part1", post(dkg_part1))
        .route("/dkg/part2", post(dkg_part2))
        .route("/dkg/part3", post(dkg_part3))
        .route("/sign/round1", post(sign_round1))
        .route("/sign/round2", post(sign_round2))
        .route("/public-key", get(get_public_key))
        .layer(cors)
        .with_state(state);

    let addr = SocketAddr::from(([0, 0, 0, 0], port));
    tracing::info!("Enclave HTTP server listening on {}", addr);

    let listener = tokio::net::TcpListener::bind(addr).await?;
    axum::serve(listener, app).await?;

    Ok(())
}
