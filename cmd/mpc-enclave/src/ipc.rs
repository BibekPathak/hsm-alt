//! HTTP Server Implementation using Axum
//! Handles communication between Go Node and Rust Enclave via HTTP/JSON

use crate::crypto::{self, DKGOutput, EpochState, KeyShare, PartialSignature, SignRound1Output};
use axum::{
    extract::State,
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

// Session timeout constants (in seconds)
const SIGNING_SESSION_TIMEOUT_SECS: u64 = 60;   // 60 seconds max
const DKG_SESSION_TIMEOUT_SECS: u64 = 600;       // 10 minutes max
const REFRESH_SESSION_TIMEOUT_SECS: u64 = 300;    // 5 minutes max

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

// Signing session state
#[derive(Debug, Clone)]
pub struct SigningSession {
    pub session_id: String,
    pub message_hash: Vec<u8>,
    pub participants: Vec<u32>,
    pub start_time: u64,
    pub nonces: Option<Vec<u8>>,
    pub commitments: HashMap<u32, Vec<u8>>,
    pub round: u8,  // 1 or 2
    pub completed: bool,
}

// DKG session state
#[derive(Debug, Clone)]
pub struct DKGSession {
    pub session_id: String,
    pub participants: Vec<u32>,
    pub start_time: u64,
    pub min_signers: u16,
    pub max_signers: u16,
    pub round: u8,  // 1, 2, or 3
    pub secret_pkg1: Option<Vec<u8>>,
    pub secret_pkg2: Option<Vec<u8>>,
    pub completed: bool,
}

pub struct Enclave {
    node_id: u32,
    cluster_id: RwLock<Option<String>>,
    threshold: u32,
    total_shares: u32,
    data_dir: RwLock<Option<String>>,
    state: RwLock<EnclaveState>,

    dkg_output: RwLock<Option<DKGOutput>>,
    my_share: RwLock<Option<KeyShare>>,
    
    // Full key packages for signing
    key_package: RwLock<Option<Vec<u8>>>,
    pubkey_package: RwLock<Option<Vec<u8>>>,
    
    // Session state
    current_epoch: RwLock<u32>,
    epoch_state: RwLock<Option<EpochState>>,
    
    // Signing session
    signing_session: RwLock<Option<SigningSession>>,
    
    // DKG session
    dkg_session: RwLock<Option<DKGSession>>,
}

impl Enclave {
    pub fn new(node_id: u32, threshold: u32, total_shares: u32) -> Self {
        Self {
            node_id,
            cluster_id: RwLock::new(None),
            threshold,
            total_shares,
            data_dir: RwLock::new(None),
            state: RwLock::new(EnclaveState::Uninitialized),
            dkg_output: RwLock::new(None),
            my_share: RwLock::new(None),
            key_package: RwLock::new(None),
            pubkey_package: RwLock::new(None),
            current_epoch: RwLock::new(0),
            epoch_state: RwLock::new(None),
            signing_session: RwLock::new(None),
            dkg_session: RwLock::new(None),
        }
    }

    fn get_current_time(&self) -> u64 {
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_secs()
    }

    pub fn set_data_dir(&self, dir: String) {
        *self.data_dir.write() = Some(dir);
    }

    fn derive_encryption_key(&self) -> Vec<u8> {
        use std::collections::hash_map::DefaultHasher;
        use std::hash::{Hash, Hasher};
        
        let node_id = self.node_id;
        let threshold = self.threshold;
        let total = self.total_shares;
        
        let mut hasher = DefaultHasher::new();
        node_id.hash(&mut hasher);
        threshold.hash(&mut hasher);
        total.hash(&mut hasher);
        "hsm-state-encryption-key-v1".hash(&mut hasher);
        
        let hash = hasher.finish();
        hash.to_le_bytes().to_vec()
    }

    pub fn attest(&self) -> (Vec<u8>, Vec<u8>, bool) {
        use std::collections::hash_map::DefaultHasher;
        use std::hash::{Hash, Hasher};
        
        let mut hasher = DefaultHasher::new();
        self.node_id.hash(&mut hasher);
        self.threshold.hash(&mut hasher);
        self.total_shares.hash(&mut hasher);
        "mpc-enclave-v1".hash(&mut hasher);
        
        let measurement = hasher.finish().to_le_bytes();
        
        let mut quote = Vec::new();
        quote.extend_from_slice(b"SGX_SIMULATION_");
        quote.extend_from_slice(&measurement);
        quote.extend_from_slice(&self.node_id.to_le_bytes());
        
        let is_simulation = true;
        
        tracing::info!("Generated attestation - NodeID: {}, Measurement: {:x?}", 
            self.node_id, measurement);
        
        (quote, measurement.to_vec(), is_simulation)
    }

    fn encrypt_data(&self, data: &[u8]) -> Vec<u8> {
        let key = self.derive_encryption_key();
        let key_arr: [u8; 32] = {
            let mut arr = [0u8; 32];
            let key_len = key.len().min(32);
            arr[..key_len].copy_from_slice(&key[..key_len]);
            arr
        };
        
        use std::collections::HashMap;
        let mut result = Vec::with_capacity(data.len() + 32);
        
        for (i, byte) in data.iter().enumerate() {
            result.push(byte ^ key_arr[i % 32]);
        }
        
        result
    }

    fn decrypt_data(&self, data: &[u8]) -> Vec<u8> {
        self.encrypt_data(data)
    }

    fn get_state_path(&self, filename: &str) -> Option<std::path::PathBuf> {
        self.data_dir.read().as_ref().map(|d| std::path::Path::new(d).join(filename))
    }

    pub fn save_state(&self) -> Result<(), String> {
        let dir_str = {
            let guard = self.data_dir.read();
            guard.as_ref().map(|d| d.to_string()).ok_or("Data directory not set")?
        };
        
        std::fs::create_dir_all(&dir_str).map_err(|e| format!("Failed to create data dir: {}", e))?;

        if let Some(my_share) = self.my_share.read().as_ref() {
            let path = std::path::Path::new(&dir_str).join("key_share.json.enc");
            let json = serde_json::to_vec(my_share).map_err(|e| e.to_string())?;
            let encrypted = self.encrypt_data(&json);
            std::fs::write(&path, &encrypted).map_err(|e| format!("Failed to write key share: {}", e))?;
            tracing::info!("Saved encrypted key share to {:?}", path);
        }

        if let Some(pubkey_pkg) = self.pubkey_package.read().as_ref() {
            let path = std::path::Path::new(&dir_str).join("pubkey_package.json");
            std::fs::write(&path, pubkey_pkg).map_err(|e| format!("Failed to write pubkey package: {}", e))?;
            tracing::info!("Saved pubkey package to {:?}", path);
        }

        if let Some(dkg_out) = self.dkg_output.read().as_ref() {
            let path = std::path::Path::new(&dir_str).join("dkg_output.json.enc");
            let json = serde_json::to_vec(dkg_out).map_err(|e| e.to_string())?;
            let encrypted = self.encrypt_data(&json);
            std::fs::write(&path, &encrypted).map_err(|e| format!("Failed to write DKG output: {}", e))?;
            tracing::info!("Saved encrypted DKG output to {:?}", path);
        }

        Ok(())
    }

    pub fn load_state(&self) -> Result<bool, String> {
        let dir_str = {
            let guard = self.data_dir.read();
            match guard.as_ref() {
                Some(d) => d.to_string(),
                None => return Ok(false),
            }
        };

        let key_share_path = std::path::Path::new(&dir_str).join("key_share.json.enc");
        if key_share_path.exists() {
            let encrypted = std::fs::read(&key_share_path).map_err(|e| format!("Failed to read key share: {}", e))?;
            let decrypted = self.decrypt_data(&encrypted);
            let share: KeyShare = serde_json::from_slice(&decrypted).map_err(|e| format!("Failed to parse key share: {}", e))?;
            *self.my_share.write() = Some(share);
            tracing::info!("Loaded and decrypted key share from {:?}", key_share_path);
        }

        let pubkey_path = std::path::Path::new(&dir_str).join("pubkey_package.json");
        if pubkey_path.exists() {
            let data = std::fs::read(&pubkey_path).map_err(|e| format!("Failed to read pubkey package: {}", e))?;
            *self.pubkey_package.write() = Some(data);
            tracing::info!("Loaded pubkey package from {:?}", pubkey_path);
        }

        let dkg_path = std::path::Path::new(&dir_str).join("dkg_output.json.enc");
        if dkg_path.exists() {
            let encrypted = std::fs::read(&dkg_path).map_err(|e| format!("Failed to read DKG output: {}", e))?;
            let decrypted = self.decrypt_data(&encrypted);
            let output: DKGOutput = serde_json::from_slice(&decrypted).map_err(|e| format!("Failed to parse DKG output: {}", e))?;
            *self.dkg_output.write() = Some(output);
            tracing::info!("Loaded and decrypted DKG output from {:?}", dkg_path);
        }

        if self.my_share.read().is_some() {
            return Ok(true);
        }

        Ok(false)
    }

    fn invalidate_signing_session(&self) {
        let mut session = self.signing_session.write();
        if session.is_some() {
            tracing::info!("Invalidating signing session");
        }
        *session = None;
    }

    fn invalidate_dkg_session(&self) {
        let mut session = self.dkg_session.write();
        if session.is_some() {
            tracing::info!("Invalidating DKG session");
        }
        *session = None;
    }

    pub fn initialize(&self, cluster_id: String) -> Result<(), String> {
        *self.cluster_id.write() = Some(cluster_id);
        
        // Try to load existing state from disk
        match self.load_state() {
            Ok(true) => {
                tracing::info!("Loaded existing key state from disk");
                *self.state.write() = EnclaveState::Ready;
            }
            Ok(false) => {
                tracing::info!("No existing state found, starting fresh");
                *self.state.write() = EnclaveState::Ready;
            }
            Err(e) => {
                tracing::warn!("Failed to load state: {}", e);
                *self.state.write() = EnclaveState::Ready;
            }
        }
        
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

    pub fn dkg_start_session(&self, session_id: String, min_signers: u16, max_signers: u16, participants: Vec<u32>) -> Result<DkgStartResult, String> {
        // Invalidate any existing DKG session
        self.invalidate_dkg_session();

        if !matches!(*self.state.read(), EnclaveState::Ready) {
            return Err("Node not ready for DKG".to_string());
        }

        let now = self.get_current_time();

        // Create new DKG session
        let session = DKGSession {
            session_id: session_id.clone(),
            participants,
            start_time: now,
            min_signers,
            max_signers,
            round: 1,
            secret_pkg1: None,
            secret_pkg2: None,
            completed: false,
        };

        *self.dkg_session.write() = Some(session);
        *self.state.write() = EnclaveState::KeyGeneration;

        tracing::info!("Started DKG session: {}", session_id);

        Ok(DkgStartResult {
            success: true,
            round: 1,
            round1_data: vec![],
        })
    }

    pub fn dkg_part1(&self, session_id: &str, min_signers: u16, max_signers: u16) -> Result<DKGPart1Result, String> {
        let now = self.get_current_time();
        
        // Check and update session
        {
            let mut session_guard = self.dkg_session.write();
            match session_guard.as_mut() {
                Some(session) if session.session_id == session_id => {
                    // Check timeout
                    if now.saturating_sub(session.start_time) > DKG_SESSION_TIMEOUT_SECS {
                        tracing::warn!("DKG session {} timed out", session_id);
                        *session_guard = None;
                        return Err("DKG session timed out".to_string());
                    }
                    if session.round != 1 {
                        return Err("Wrong round for DKG part1".to_string());
                    }
                }
                Some(_) => {
                    return Err("Different session already in progress".to_string());
                }
                None => {
                    // No session, create new one
                    *session_guard = Some(DKGSession {
                        session_id: session_id.to_string(),
                        participants: vec![],
                        start_time: now,
                        min_signers,
                        max_signers,
                        round: 1,
                        secret_pkg1: None,
                        secret_pkg2: None,
                        completed: false,
                    });
                }
            }
        }

        *self.state.write() = EnclaveState::KeyGeneration;

        let output = crypto::dkg_part1(self.node_id, max_signers, min_signers)
            .map_err(|e| e.to_string())?;

        // Store secret package in session
        {
            let mut session_guard = self.dkg_session.write();
            if let Some(session) = session_guard.as_mut() {
                session.secret_pkg1 = Some(output.secret_package.clone());
            }
        }

        Ok(DKGPart1Result {
            success: true,
            error: String::new(),
            secret_package: output.secret_package,
            round1_package: output.package,
        })
    }

    pub fn dkg_part2(&self, session_id: &str, round1_packages: BTreeMap<u32, Vec<u8>>) -> Result<DKGPart2Result, String> {
        let now = self.get_current_time();

        // Verify session and check timeout
        let secret_pkg1 = {
            let mut session_guard = self.dkg_session.write();
            match session_guard.as_ref() {
                Some(session) if session.session_id == session_id => {
                    if now.saturating_sub(session.start_time) > DKG_SESSION_TIMEOUT_SECS {
                        tracing::warn!("DKG session {} timed out", session_id);
                        *session_guard = None;
                        return Err("DKG session timed out".to_string());
                    }
                    if session.round != 1 {
                        return Err("Wrong round for DKG part2".to_string());
                    }
                    session.secret_pkg1.clone()
                }
                Some(_) => return Err("Different session in progress".to_string()),
                None => return Err("No DKG session found".to_string()),
            }
        };

        let secret_pkg1 = secret_pkg1.ok_or("No secret package from round 1")?;

        let output = crypto::dkg_part2(&secret_pkg1, &round1_packages)
            .map_err(|e| e.to_string())?;

        // Update session
        {
            let mut session_guard = self.dkg_session.write();
            if let Some(session) = session_guard.as_mut() {
                session.round = 2;
                session.secret_pkg2 = Some(output.secret_package.clone());
            }
        }

        Ok(DKGPart2Result {
            success: true,
            error: String::new(),
            secret_package: output.secret_package,
            round2_packages: output.packages,
        })
    }

    pub fn dkg_part3(
        &self,
        session_id: &str,
        round1_packages: BTreeMap<u32, Vec<u8>>,
        round2_packages: BTreeMap<u32, Vec<u8>>,
    ) -> Result<DKGPart3Result, String> {
        let now = self.get_current_time();

        // Verify session and check timeout
        let secret_pkg2 = {
            let mut session_guard = self.dkg_session.write();
            match session_guard.as_ref() {
                Some(session) if session.session_id == session_id => {
                    if now.saturating_sub(session.start_time) > DKG_SESSION_TIMEOUT_SECS {
                        tracing::warn!("DKG session {} timed out", session_id);
                        *session_guard = None;
                        return Err("DKG session timed out".to_string());
                    }
                    if session.round != 2 {
                        return Err("Wrong round for DKG part3".to_string());
                    }
                    session.secret_pkg2.clone()
                }
                Some(_) => return Err("Different session in progress".to_string()),
                None => return Err("No DKG session found".to_string()),
            }
        };

        let secret_pkg2 = secret_pkg2.ok_or("No secret package from round 2")?;

        let output = crypto::dkg_part3(&secret_pkg2, &round1_packages, &round2_packages)
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

        // Save state to disk
        if let Err(e) = self.save_state() {
            tracing::warn!("Failed to save state: {}", e);
        }

        // Complete and invalidate session
        {
            let mut session_guard = self.dkg_session.write();
            if let Some(session) = session_guard.as_mut() {
                session.completed = true;
                tracing::info!("DKG session {} completed successfully", session_id);
            }
            *session_guard = None;
        }

        Ok(DKGPart3Result {
            success: true,
            error: String::new(),
            key_package: output.key_package,
            pubkey_package: output.pubkey_package,
        })
    }

    pub fn dkg_abort(&self, session_id: &str) -> Result<(), String> {
        let session_guard = self.dkg_session.read();
        match session_guard.as_ref() {
            Some(session) if session.session_id == session_id => {
                tracing::info!("Aborting DKG session: {}", session_id);
                drop(session_guard);
                self.invalidate_dkg_session();
                *self.state.write() = EnclaveState::Ready;
                Ok(())
            }
            Some(_) => Err("Different session in progress".to_string()),
            None => Err("No DKG session found".to_string()),
        }
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

    pub fn get_pubkey_package(&self) -> Result<Vec<u8>, String> {
        let pkg = self.pubkey_package.read();
        match pkg.as_ref() {
            Some(p) => Ok(p.clone()),
            None => Err("Public key package not available".to_string()),
        }
    }

    pub fn aggregate_signatures(&self, message: Vec<u8>, partial_signatures: BTreeMap<u32, Vec<u8>>) -> Result<Vec<u8>, String> {
        let pubkey_pkg = self.pubkey_package.read();
        let pubkey_package = pubkey_pkg.as_ref().ok_or("Public key package not available")?;

        crypto::aggregate_signatures(&message, &partial_signatures, pubkey_package)
            .map_err(|e| e.to_string())
    }

    pub fn verify_signature(&self, signature: Vec<u8>, message: Vec<u8>) -> Result<bool, String> {
        let pubkey_pkg = self.pubkey_package.read();
        let pubkey_package = pubkey_pkg.as_ref().ok_or("Public key package not available")?;

        crypto::verify_signature(&signature, &message, pubkey_package)
            .map_err(|e| e.to_string())
    }

    pub fn sign_start_session(&self, session_id: String, message: Vec<u8>, participants: Vec<u32>) -> Result<(), String> {
        // Invalidate any existing signing session
        self.invalidate_signing_session();

        if self.my_share.read().is_none() {
            return Err("Key share not available".to_string());
        }

        let now = self.get_current_time();

        // Create new signing session
        let session = SigningSession {
            session_id: session_id.clone(),
            message_hash: crypto::hash_message(&message),
            participants,
            start_time: now,
            nonces: None,
            commitments: HashMap::new(),
            round: 1,
            completed: false,
        };

        *self.signing_session.write() = Some(session);
        *self.state.write() = EnclaveState::Signing;

        tracing::info!("Started signing session: {}", session_id);

        Ok(())
    }

    pub fn sign_round1(&self, session_id: &str) -> Result<SignRound1Result, String> {
        let now = self.get_current_time();

        // Verify session and check timeout
        {
            let mut session_guard = self.signing_session.write();
            match session_guard.as_mut() {
                Some(session) if session.session_id == session_id => {
                    if now.saturating_sub(session.start_time) > SIGNING_SESSION_TIMEOUT_SECS {
                        tracing::warn!("Signing session {} timed out", session_id);
                        *session_guard = None;
                        *self.state.write() = EnclaveState::Ready;
                        return Err("Signing session timed out".to_string());
                    }
                    if session.round != 1 {
                        return Err("Wrong round for sign round1".to_string());
                    }
                }
                Some(_) => return Err("Different signing session in progress".to_string()),
                None => return Err("No signing session found".to_string()),
            }
        }

        let share = self.my_share.read();
        let key_package = share.as_ref().ok_or("Key share not available")?;

        let output = crypto::sign_round1(&key_package.key_package).map_err(|e| e.to_string())?;

        // Store nonces in session
        {
            let mut session_guard = self.signing_session.write();
            if let Some(session) = session_guard.as_mut() {
                session.nonces = Some(output.nonces.clone());
            }
        }

        Ok(SignRound1Result {
            success: true,
            error: String::new(),
            nonce_commitment: output.nonces,
            commitment: output.commitment,
        })
    }

    pub fn add_commitment(&self, session_id: &str, from_node: u32, commitment: Vec<u8>) -> Result<(), String> {
        let mut session_guard = self.signing_session.write();
        match session_guard.as_mut() {
            Some(session) if session.session_id == session_id => {
                session.commitments.insert(from_node, commitment);
                Ok(())
            }
            Some(_) => Err("Different signing session in progress".to_string()),
            None => Err("No signing session found".to_string()),
        }
    }

    pub fn sign_round2(&self, session_id: &str, signing_package_bytes: Vec<u8>) -> Result<SignRound2Result, String> {
        let now = self.get_current_time();

        // Verify session and check timeout
        let (nonces, message_hash) = {
            let mut session_guard = self.signing_session.write();
            match session_guard.as_mut() {
                Some(session) if session.session_id == session_id => {
                    if now.saturating_sub(session.start_time) > SIGNING_SESSION_TIMEOUT_SECS {
                        tracing::warn!("Signing session {} timed out", session_id);
                        *session_guard = None;
                        *self.state.write() = EnclaveState::Ready;
                        return Err("Signing session timed out".to_string());
                    }
                    if session.round != 1 {
                        return Err("Wrong round for sign round2".to_string());
                    }
                    session.round = 2;
                    (session.nonces.clone(), session.message_hash.clone())
                }
                Some(_) => return Err("Different signing session in progress".to_string()),
                None => return Err("No signing session found".to_string()),
            }
        };

        let nonces = nonces.ok_or("No nonces from round 1")?;

        let share = self.my_share.read();
        let key_package = share.as_ref().ok_or("Key share not available")?;

        let commitments = {
            let session_guard = self.signing_session.read();
            match session_guard.as_ref() {
                Some(session) => session.commitments.clone(),
                None => return Err("No signing session found".to_string()),
            }
        };

        let partial = crypto::sign_round2(
            &message_hash,
            &nonces,
            &key_package.key_package,
            &commitments,
        )
        .map_err(|e| e.to_string())?;

        *self.state.write() = EnclaveState::Ready;

        // Complete and invalidate session
        {
            let mut session_guard = self.signing_session.write();
            if let Some(session) = session_guard.as_mut() {
                session.completed = true;
                tracing::info!("Signing session {} completed", session_id);
            }
            *session_guard = None;
        }

        Ok(SignRound2Result {
            success: true,
            error: String::new(),
            partial_signature: partial.signature_share,
            commitment: partial.commitment,
        })
    }

    pub fn sign_abort(&self, session_id: &str) -> Result<(), String> {
        let session_guard = self.signing_session.read();
        match session_guard.as_ref() {
            Some(session) if session.session_id == session_id => {
                tracing::info!("Aborting signing session: {}", session_id);
                drop(session_guard);
                self.invalidate_signing_session();
                *self.state.write() = EnclaveState::Ready;
                Ok(())
            }
            Some(_) => Err("Different signing session in progress".to_string()),
            None => Err("No signing session found".to_string()),
        }
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
    session_id: String,
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
    session_id: String,
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
    session_id: String,
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
struct SignRound1Request {
    session_id: String,
}

#[derive(Debug, Serialize, Deserialize)]
struct SignStartRequest {
    session_id: String,
    message: Vec<u8>,
    participants: Vec<u32>,
}

#[derive(Debug, Serialize, Deserialize)]
struct SignStartResult {
    success: bool,
    error: String,
}

#[derive(Debug, Serialize, Deserialize)]
struct SignRound1Result {
    success: bool,
    error: String,
    nonce_commitment: Vec<u8>,
    commitment: Vec<u8>,
}

#[derive(Debug, Serialize, Deserialize)]
struct SignRound2Request {
    session_id: String,
    signing_package: Vec<u8>,
}

#[derive(Debug, Serialize, Deserialize)]
struct SignRound2Result {
    success: bool,
    error: String,
    partial_signature: Vec<u8>,
    commitment: Vec<u8>,
}

#[derive(Debug, Serialize, Deserialize)]
struct PublicKeyResponse {
    public_key: Vec<u8>,
}

#[derive(Debug, Serialize, Deserialize)]
struct AggregateRequest {
    message: Vec<u8>,
    partial_signatures: BTreeMap<u32, Vec<u8>>,
}

#[derive(Debug, Serialize, Deserialize)]
struct AggregateResponse {
    success: bool,
    error: String,
    signature: Vec<u8>,
}

#[derive(Debug, Serialize, Deserialize)]
struct VerifyRequest {
    signature: Vec<u8>,
    message: Vec<u8>,
}

#[derive(Debug, Serialize, Deserialize)]
struct VerifyResponse {
    valid: bool,
    error: String,
}

#[derive(Debug, Serialize, Deserialize)]
struct AttestResponse {
    quote: Vec<u8>,
    measurement: Vec<u8>,
    is_simulation: bool,
}

#[derive(Debug, Serialize, Deserialize)]
struct SignAbortRequest {
    session_id: String,
}

#[derive(Debug, Serialize, Deserialize)]
struct SignAbortResponse {
    success: bool,
    error: String,
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
    match enclave.dkg_part1(&req.session_id, req.min_signers as u16, req.max_signers as u16) {
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
    match enclave.dkg_part2(&req.session_id, req.round1_packages) {
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
    match enclave.dkg_part3(&req.session_id, req.round1_packages, req.round2_packages) {
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

async fn sign_start(
    State(state): State<AppState>,
    Json(req): Json<SignStartRequest>,
) -> Json<SignStartResult> {
    let enclave = state.enclave.read();
    match enclave.sign_start_session(req.session_id, req.message, req.participants) {
        Ok(()) => Json(SignStartResult {
            success: true,
            error: String::new(),
        }),
        Err(e) => Json(SignStartResult {
            success: false,
            error: e,
        }),
    }
}

async fn sign_round1(
    State(state): State<AppState>,
    Json(req): Json<SignRound1Request>,
) -> Json<SignRound1Result> {
    let enclave = state.enclave.read();
    match enclave.sign_round1(&req.session_id) {
        Ok(result) => Json(SignRound1Result {
            success: true,
            error: String::new(),
            nonce_commitment: result.nonce_commitment,
            commitment: result.commitment,
        }),
        Err(e) => Json(SignRound1Result {
            success: false,
            error: e,
            nonce_commitment: vec![],
            commitment: vec![],
        }),
    }
}

async fn sign_round2(
    State(state): State<AppState>,
    Json(req): Json<SignRound2Request>,
) -> Json<SignRound2Result> {
    let enclave = state.enclave.read();
    match enclave.sign_round2(&req.session_id, req.signing_package) {
        Ok(result) => Json(SignRound2Result {
            success: true,
            error: String::new(),
            partial_signature: result.partial_signature,
            commitment: result.commitment,
        }),
        Err(e) => Json(SignRound2Result {
            success: false,
            error: e,
            partial_signature: vec![],
            commitment: vec![],
        }),
    }
}

async fn aggregate_signatures(
    State(state): State<AppState>,
    Json(req): Json<AggregateRequest>,
) -> Json<AggregateResponse> {
    let enclave = state.enclave.read();
    match enclave.aggregate_signatures(req.message, req.partial_signatures) {
        Ok(sig) => Json(AggregateResponse {
            success: true,
            error: String::new(),
            signature: sig,
        }),
        Err(e) => Json(AggregateResponse {
            success: false,
            error: e,
            signature: vec![],
        }),
    }
}

async fn verify_signature(State(state): State<AppState>, Json(req): Json<VerifyRequest>) -> Json<VerifyResponse> {
    let enclave = state.enclave.read();
    match enclave.verify_signature(req.signature, req.message) {
        Ok(valid) => Json(VerifyResponse { valid, error: String::new() }),
        Err(e) => Json(VerifyResponse { valid: false, error: e }),
    }
}

async fn sign_abort(State(state): State<AppState>, Json(req): Json<SignAbortRequest>) -> Json<SignAbortResponse> {
    let enclave = state.enclave.read();
    match enclave.sign_abort(&req.session_id) {
        Ok(()) => Json(SignAbortResponse {
            success: true,
            error: String::new(),
        }),
        Err(e) => Json(SignAbortResponse {
            success: false,
            error: e,
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

async fn attest(State(state): State<AppState>) -> Json<AttestResponse> {
    let enclave = state.enclave.read();
    let (quote, measurement, is_simulation) = enclave.attest();
    Json(AttestResponse {
        quote,
        measurement,
        is_simulation,
    })
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
        .route("/sign/start", post(sign_start))
        .route("/sign/round1", post(sign_round1))
        .route("/sign/round2", post(sign_round2))
        .route("/sign/abort", post(sign_abort))
        .route("/aggregate", post(aggregate_signatures))
        .route("/verify", post(verify_signature))
        .route("/public-key", get(get_public_key))
        .route("/attest", get(attest))
        .layer(cors)
        .with_state(state);

    let addr = SocketAddr::from(([0, 0, 0, 0], port));
    tracing::info!("Enclave HTTP server listening on {}", addr);

    let listener = tokio::net::TcpListener::bind(addr).await?;
    axum::serve(listener, app).await?;

    Ok(())
}
