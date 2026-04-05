package ethereum

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// DeriveAddressFromPublicKey derives an Ethereum address from an uncompressed public key
func DeriveAddressFromPublicKey(pubKey []byte) (string, error) {
	if len(pubKey) == 0 {
		return "", fmt.Errorf("empty public key")
	}

	var ecdsaPubKey *ecdsa.PublicKey
	var err error

	if len(pubKey) == 65 {
		// Uncompressed public key (65 bytes with 0x04 prefix)
		ecdsaPubKey, err = crypto.UnmarshalPubkey(pubKey)
		if err != nil {
			return "", fmt.Errorf("failed to unmarshal uncompressed public key: %w", err)
		}
	} else if len(pubKey) == 33 {
		// Compressed public key (33 bytes)
		ecdsaPubKey, err = crypto.DecompressPubkey(pubKey)
		if err != nil {
			return "", fmt.Errorf("failed to decompress public key: %w", err)
		}
	} else {
		return "", fmt.Errorf("invalid public key length: %d (expected 33 or 65)", len(pubKey))
	}

	// Derive address: Keccak256(pubKey)[12:]
	address := crypto.PubkeyToAddress(*ecdsaPubKey)
	return address.Hex(), nil
}

// DeriveAddressFromBytes derives an Ethereum address from raw uncompressed pubkey bytes
func DeriveAddressFromBytes(pubKey []byte) (string, error) {
	return DeriveAddressFromPublicKey(pubKey)
}

// ValidateAddress checks if a string is a valid Ethereum address
func ValidateAddress(address string) bool {
	return common.IsHexAddress(address)
}

// ToChecksumAddress converts an address to EIP-55 checksummed format
func ToChecksumAddress(address string) string {
	if !ValidateAddress(address) {
		return ""
	}
	return common.HexToAddress(address).Hex()
}

// GetAddressFromPrivate derives an address directly from a private key
func GetAddressFromPrivate(privateKey *ecdsa.PrivateKey) string {
	return crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
}

// GenerateAddress generates a new keypair and returns the address
func GenerateAddress() (string, []byte, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate key: %w", err)
	}

	address := crypto.PubkeyToAddress(key.PublicKey).Hex()
	pubKey := crypto.FromECDSAPub(&key.PublicKey)

	return address, pubKey, nil
}
