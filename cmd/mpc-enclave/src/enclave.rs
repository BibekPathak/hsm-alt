use crate::crypto;
use crate::crypto::{EpochState, KeyShare, ThresholdConfig};
use std::sync::RwLock;
use thiserror::Error;

#[derive(Error, Debug)]
pub enum EnclaveError {
    #[error("Enclave not initialized")]
    NotInitialized,
    #[error("Invalid state: {0}")]
    InvalidState(String),
    #[error("Operation failed: {0}")]
    OperationFailed(String),
}

#[derive(Debug, Clone)]
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
    config: ThresholdConfig,
    node_id: u32,
    cluster_id: RwLock<Option<String>>,
    state: RwLock<EnclaveState>,
    public_key: RwLock<Option<Vec<u8>>>,
    key_share: RwLock<Option<KeyShare>>,
}

impl Enclave {
    pub fn new(node_id: u32, threshold: u32, total_shares: u32) -> Self {
        let config = ThresholdConfig {
            threshold,
            total_shares,
        };

        Self {
            config,
            node_id,
            cluster_id: RwLock::new(None),
            state: RwLock::new(EnclaveState::Uninitialized),
            public_key: RwLock::new(None),
            key_share: RwLock::new(None),
        }
    }

    pub fn initialize(&self, cluster_id: String) -> Result<(), EnclaveError> {
        *self.cluster_id.write().unwrap() = Some(cluster_id);
        *self.state.write().unwrap() = EnclaveState::Ready;
        Ok(())
    }

    pub fn start_dkg(
        &self,
        _participants: Vec<u32>,
    ) -> Result<(Vec<u8>, Vec<Vec<u8>>), EnclaveError> {
        let share = crypto::generate_key_share();
        let public_key = crypto::compute_public_key(&share);

        *self.key_share.write().unwrap() = Some(share);
        *self.public_key.write().unwrap() = Some(public_key.clone());

        Ok((vec![], vec![public_key]))
    }

    pub fn sign(&self, _message: Vec<u8>, _signers: Vec<u32>) -> Result<Vec<u8>, EnclaveError> {
        let share = self.key_share.read().unwrap();
        match share.as_ref() {
            Some(s) => Ok(crypto::sign(s, b"message")),
            None => Err(EnclaveError::NotInitialized),
        }
    }

    pub fn start_resharing(
        &self,
        _new_threshold: u32,
        _new_total: u32,
    ) -> Result<(), EnclaveError> {
        Ok(())
    }

    pub fn evolve_key(&self) -> Result<EpochState, EnclaveError> {
        Ok(EpochState {
            epoch: 1,
            public_key: vec![],
            chain_code: vec![],
            commitment: vec![],
        })
    }

    pub fn get_public_key(&self) -> Result<Vec<u8>, EnclaveError> {
        let pk = self.public_key.read().unwrap();
        pk.clone().ok_or(EnclaveError::NotInitialized)
    }

    pub fn get_current_epoch(&self) -> u32 {
        1
    }

    pub fn get_state(&self) -> EnclaveState {
        self.state.read().unwrap().clone()
    }

    pub fn is_initialized(&self) -> bool {
        matches!(*self.state.read().unwrap(), EnclaveState::Ready)
    }
}
