use curve25519_dalek::edwards::EdwardsPoint;
use curve25519_dalek::traits::Identity;
use frost::keys::dkg::{part1, part2, part3, round1 as dkg_round1, round2 as dkg_round2};
use frost::keys::{generate_with_dealer, IdentifierList, KeyPackage, PublicKeyPackage};
use frost::round1::commit;
use frost::round2::sign;
use frost::Identifier;
use frost::{Signature, SigningPackage};
use frost_ed25519 as frost;
use rand::rngs::OsRng;
use rand::RngCore;
use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, HashMap};
use thiserror::Error;

#[derive(Error, Debug)]
pub enum CryptoError {
    #[error("Key generation failed: {0}")]
    KeyGenError(String),
    #[error("DKG failed: {0}")]
    DKGError(String),
    #[error("Signing failed: {0}")]
    SigningError(String),
    #[error("Verification failed: {0}")]
    VerificationError(String),
    #[error("Serialization error: {0}")]
    SerializationError(String),
    #[error("Not initialized")]
    NotInitialized,
    #[error("Invalid state: {0}")]
    InvalidState(String),
}

pub type Result<T> = std::result::Result<T, CryptoError>;

impl From<serde_json::Error> for CryptoError {
    fn from(e: serde_json::Error) -> Self {
        CryptoError::SerializationError(e.to_string())
    }
}

#[derive(Clone, Serialize, Deserialize)]
pub struct KeyShare {
    pub index: u32,
    pub key_package: Vec<u8>,
}

#[derive(Clone, Serialize, Deserialize)]
pub struct PublicKey {
    pub key: Vec<u8>,
}

#[derive(Clone, Serialize, Deserialize)]
pub struct DKGOutput {
    pub shares: Vec<KeyShare>,
    pub public_key: PublicKey,
    pub public_key_package: Vec<u8>,
}

#[derive(Clone, Serialize, Deserialize)]
pub struct SignRound1Output {
    pub nonces: Vec<u8>,
    pub commitment: Vec<u8>,
}

#[derive(Clone, Serialize, Deserialize)]
pub struct PartialSignature {
    pub signature_share: Vec<u8>,
    pub commitment: Vec<u8>,
}

#[derive(Clone, Serialize, Deserialize)]
pub struct EpochState {
    pub epoch: u32,
    pub public_key: Vec<u8>,
    pub chain_code: Vec<u8>,
    pub commitment: Vec<u8>,
}

#[derive(Clone, Serialize, Deserialize)]
pub struct KeyPackageSerde {
    pub serialized: Vec<u8>,
    pub index: u32,
}

#[derive(Clone, Serialize, Deserialize)]
pub struct PublicKeyPackageSerde {
    pub serialized: Vec<u8>,
}

#[derive(Clone, Serialize, Deserialize)]
pub struct DKGPart1Output {
    pub secret_package: Vec<u8>,
    pub package: Vec<u8>,
}

#[derive(Clone, Serialize, Deserialize)]
pub struct DKGPart2Output {
    pub secret_package: Vec<u8>,
    pub packages: BTreeMap<u32, Vec<u8>>,
}

#[derive(Clone, Serialize, Deserialize)]
pub struct DKGPart3Output {
    pub key_package: Vec<u8>,
    pub pubkey_package: Vec<u8>,
}

pub fn dkg_part1(node_id: u32, max_signers: u16, min_signers: u16) -> Result<DKGPart1Output> {
    let identifier = Identifier::try_from(node_id as u16)
        .map_err(|e| CryptoError::DKGError(format!("Failed to create identifier: {:?}", e)))?;

    let (secret_package, package) = part1(identifier, max_signers, min_signers, &mut OsRng)
        .map_err(|e| CryptoError::DKGError(format!("DKG part1 failed: {:?}", e)))?;

    Ok(DKGPart1Output {
        secret_package: serde_json::to_vec(&secret_package)?,
        package: serde_json::to_vec(&package)?,
    })
}

pub fn dkg_part2(
    secret_package_bytes: &[u8],
    round1_packages_bytes: &BTreeMap<u32, Vec<u8>>,
) -> Result<DKGPart2Output> {
    let secret_package: dkg_round1::SecretPackage = serde_json::from_slice(secret_package_bytes)
        .map_err(|e| {
            CryptoError::DKGError(format!("Failed to deserialize secret package: {:?}", e))
        })?;

    let mut round1_packages = BTreeMap::new();
    for (id, pkg_bytes) in round1_packages_bytes {
        let pkg: dkg_round1::Package = serde_json::from_slice(pkg_bytes).map_err(|e| {
            CryptoError::DKGError(format!("Failed to deserialize round1 package: {:?}", e))
        })?;
        let identifier = Identifier::try_from(*id as u16)
            .map_err(|e| CryptoError::DKGError(format!("Failed to create identifier: {:?}", e)))?;
        round1_packages.insert(identifier, pkg);
    }

    let (secret_package, round2_packages) = part2(secret_package, &round1_packages)
        .map_err(|e| CryptoError::DKGError(format!("DKG part2 failed: {:?}", e)))?;

    let mut packages = BTreeMap::new();
    for (id, pkg) in round2_packages {
        let id_bytes = id.serialize();
        let id_u32 = u32::from_be_bytes(
            id_bytes
                .as_slice()
                .try_into()
                .map_err(|_| CryptoError::DKGError("Failed to convert identifier".to_string()))?,
        );
        packages.insert(id_u32, serde_json::to_vec(&pkg)?);
    }

    Ok(DKGPart2Output {
        secret_package: serde_json::to_vec(&secret_package)?,
        packages,
    })
}

pub fn dkg_part3(
    secret_package_bytes: &[u8],
    round1_packages_bytes: &BTreeMap<u32, Vec<u8>>,
    round2_packages_bytes: &BTreeMap<u32, Vec<u8>>,
) -> Result<DKGPart3Output> {
    let secret_package: dkg_round2::SecretPackage = serde_json::from_slice(secret_package_bytes)
        .map_err(|e| {
            CryptoError::DKGError(format!(
                "Failed to deserialize round2 secret package: {:?}",
                e
            ))
        })?;

    let mut round1_packages = BTreeMap::new();
    for (id, pkg_bytes) in round1_packages_bytes {
        let pkg: dkg_round1::Package = serde_json::from_slice(pkg_bytes).map_err(|e| {
            CryptoError::DKGError(format!("Failed to deserialize round1 package: {:?}", e))
        })?;
        let identifier = Identifier::try_from(*id as u16)
            .map_err(|e| CryptoError::DKGError(format!("Failed to create identifier: {:?}", e)))?;
        round1_packages.insert(identifier, pkg);
    }

    let mut round2_packages = BTreeMap::new();
    for (id, pkg_bytes) in round2_packages_bytes {
        let pkg: dkg_round2::Package = serde_json::from_slice(pkg_bytes).map_err(|e| {
            CryptoError::DKGError(format!("Failed to deserialize round2 package: {:?}", e))
        })?;
        let identifier = Identifier::try_from(*id as u16)
            .map_err(|e| CryptoError::DKGError(format!("Failed to create identifier: {:?}", e)))?;
        round2_packages.insert(identifier, pkg);
    }

    let (key_package, pubkey_package) = part3(&secret_package, &round1_packages, &round2_packages)
        .map_err(|e| CryptoError::DKGError(format!("DKG part3 failed: {:?}", e)))?;

    Ok(DKGPart3Output {
        key_package: serde_json::to_vec(&key_package)?,
        pubkey_package: serde_json::to_vec(&pubkey_package)?,
    })
}

pub fn dkg_generate(min_signers: u16, max_signers: u16) -> Result<DKGOutput> {
    let mut rng = rand::thread_rng();

    let (shares_map, pubkey_package) =
        generate_with_dealer(max_signers, min_signers, IdentifierList::Default, &mut rng)
            .map_err(|e| CryptoError::KeyGenError(format!("Key generation failed: {:?}", e)))?;

    let mut key_shares = Vec::new();
    for (identifier, secret_share) in shares_map {
        let key_package = KeyPackage::try_from(secret_share).map_err(|e| {
            CryptoError::KeyGenError(format!("Failed to create key package: {:?}", e))
        })?;

        let id_bytes = identifier.serialize();
        let index =
            u32::from_be_bytes(id_bytes.as_slice().try_into().map_err(|_| {
                CryptoError::KeyGenError("Failed to convert identifier".to_string())
            })?);

        key_shares.push(KeyShare {
            index,
            key_package: serde_json::to_vec(&key_package)?,
        });
    }

    Ok(DKGOutput {
        shares: key_shares,
        public_key: PublicKey {
            key: pubkey_package
                .verifying_key()
                .serialize()
                .map_err(|e| CryptoError::SerializationError(format!("{:?}", e)))?,
        },
        public_key_package: serde_json::to_vec(&pubkey_package)?,
    })
}

pub fn sign_round1(key_package_bytes: &[u8]) -> Result<SignRound1Output> {
    let key_package: KeyPackage = serde_json::from_slice(key_package_bytes).map_err(|e| {
        CryptoError::SigningError(format!("Failed to deserialize key package: {:?}", e))
    })?;

    let signing_share = key_package.signing_share();
    let (nonces, commitments) = commit(signing_share, &mut OsRng);

    Ok(SignRound1Output {
        nonces: serde_json::to_vec(&nonces)?,
        commitment: serde_json::to_vec(&commitments)?,
    })
}

pub fn sign_round2(
    message: &[u8],
    nonces_bytes: &[u8],
    key_package_bytes: &[u8],
    all_commitments_bytes: &HashMap<u32, Vec<u8>>,
) -> Result<PartialSignature> {
    let key_package: KeyPackage = serde_json::from_slice(key_package_bytes).map_err(|e| {
        CryptoError::SigningError(format!("Failed to deserialize key package: {:?}", e))
    })?;

    let nonces: frost::round1::SigningNonces = serde_json::from_slice(nonces_bytes)
        .map_err(|e| CryptoError::SigningError(format!("Failed to deserialize nonces: {:?}", e)))?;

    let identifier = *key_package.identifier();

    let mut commitments_map = BTreeMap::new();
    for (id, commit_bytes) in all_commitments_bytes {
        let frost_id = Identifier::try_from(*id as u16).map_err(|e| {
            CryptoError::SigningError(format!("Failed to create identifier: {:?}", e))
        })?;
        let commitment: frost::round1::SigningCommitments = serde_json::from_slice(commit_bytes)
            .map_err(|e| {
                CryptoError::SigningError(format!("Failed to deserialize commitment: {:?}", e))
            })?;
        commitments_map.insert(frost_id, commitment);
    }

    let signing_package = SigningPackage::new(commitments_map, message);

    let signature_share = sign(&signing_package, &nonces, &key_package)
        .map_err(|e| CryptoError::SigningError(format!("Signing failed: {:?}", e)))?;

    let commitment = signing_package
        .signing_commitments()
        .get(&identifier)
        .cloned()
        .ok_or_else(|| CryptoError::SigningError("Commitment not found".to_string()))?;

    Ok(PartialSignature {
        signature_share: serde_json::to_vec(&signature_share)?,
        commitment: serde_json::to_vec(&commitment)?,
    })
}

pub fn aggregate_signatures(
    message: &[u8],
    signature_shares: &BTreeMap<u32, Vec<u8>>,
    public_key_package_bytes: &[u8],
) -> Result<Vec<u8>> {
    let pubkey_package: PublicKeyPackage = serde_json::from_slice(public_key_package_bytes)
        .map_err(|e| {
            CryptoError::SigningError(format!("Failed to deserialize public key package: {:?}", e))
        })?;

    let mut commitments_map = BTreeMap::new();

    for (id, _) in signature_shares {
        let frost_id = Identifier::try_from(*id as u16).map_err(|e| {
            CryptoError::SigningError(format!("Failed to create identifier: {:?}", e))
        })?;

        let commitment = frost::round1::SigningCommitments::new(
            frost::round1::NonceCommitment::new(EdwardsPoint::identity()),
            frost::round1::NonceCommitment::new(EdwardsPoint::identity()),
        );
        commitments_map.insert(frost_id, commitment);
    }

    let signing_package = SigningPackage::new(commitments_map, message);

    let mut shares_map = BTreeMap::new();
    for (id, share_bytes) in signature_shares {
        let frost_id = Identifier::try_from(*id as u16).map_err(|e| {
            CryptoError::SigningError(format!("Failed to create identifier: {:?}", e))
        })?;
        let share: frost::round2::SignatureShare =
            serde_json::from_slice(share_bytes).map_err(|e| {
                CryptoError::SigningError(format!("Failed to deserialize signature share: {:?}", e))
            })?;
        shares_map.insert(frost_id, share);
    }

    let signature = frost::aggregate(&signing_package, &shares_map, &pubkey_package)
        .map_err(|e| CryptoError::SigningError(format!("Aggregation failed: {:?}", e)))?;

    Ok(signature
        .serialize()
        .map_err(|e| CryptoError::SerializationError(format!("{:?}", e)))?)
}

pub fn verify_signature(
    signature_bytes: &[u8],
    message: &[u8],
    public_key_package_bytes: &[u8],
) -> Result<bool> {
    if signature_bytes.is_empty() || public_key_package_bytes.is_empty() {
        return Ok(false);
    }

    let pubkey_package: PublicKeyPackage = serde_json::from_slice(public_key_package_bytes)
        .map_err(|e| {
            CryptoError::VerificationError(format!(
                "Failed to deserialize public key package: {:?}",
                e
            ))
        })?;

    let signature: Signature = serde_json::from_slice(signature_bytes).map_err(|e| {
        CryptoError::VerificationError(format!("Failed to deserialize signature: {:?}", e))
    })?;

    let verifying_key = pubkey_package.verifying_key();

    match verifying_key.verify(message, &signature) {
        Ok(()) => Ok(true),
        Err(_) => Ok(false),
    }
}

pub fn get_key_share_index(key_package_bytes: &[u8]) -> Result<u32> {
    if key_package_bytes.is_empty() {
        return Err(CryptoError::NotInitialized);
    }
    let pkg: KeyPackage = serde_json::from_slice(key_package_bytes)?;
    let id_bytes = pkg.identifier().serialize();
    Ok(u32::from_be_bytes(id_bytes.as_slice().try_into().map_err(
        |_| CryptoError::SigningError("Failed to convert identifier".to_string()),
    )?))
}

pub fn get_verifying_key(public_key_package_bytes: &[u8]) -> Result<Vec<u8>> {
    if public_key_package_bytes.is_empty() {
        return Err(CryptoError::NotInitialized);
    }
    let pkg: PublicKeyPackage = serde_json::from_slice(public_key_package_bytes)?;
    Ok(pkg
        .verifying_key()
        .serialize()
        .map_err(|e| CryptoError::SerializationError(format!("{:?}", e)))?)
}

impl EpochState {
    pub fn new(initial_public_key: Vec<u8>) -> Self {
        Self {
            epoch: 1,
            public_key: initial_public_key,
            chain_code: vec![0u8; 32],
            commitment: vec![0u8; 32],
        }
    }
}

pub fn evolve_key(_current_state: &EpochState) -> Result<EpochState> {
    Err(CryptoError::InvalidState("Not implemented".to_string()))
}
