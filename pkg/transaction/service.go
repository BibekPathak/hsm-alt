package transaction

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/yourorg/hsm/pkg/blockchain/ethereum"
	"github.com/yourorg/hsm/pkg/blockchain/solana"
	"github.com/yourorg/hsm/pkg/signer"
)

type FeeSpeed string

const (
	FeeSpeedSlow     FeeSpeed = "slow"
	FeeSpeedStandard FeeSpeed = "standard"
	FeeSpeedFast     FeeSpeed = "fast"
)

type Service struct {
	ethereumBuilders map[string]*ethereum.TxBuilder
	solanaBuilders   map[string]*solana.TxBuilder
}

func NewService() *Service {
	return &Service{
		ethereumBuilders: make(map[string]*ethereum.TxBuilder),
		solanaBuilders:   make(map[string]*solana.TxBuilder),
	}
}

func (s *Service) AddChain(chain string, builder *ethereum.TxBuilder) {
	s.ethereumBuilders[chain] = builder
}

func (s *Service) AddSolanaChain(chain string, builder *solana.TxBuilder) {
	s.solanaBuilders[chain] = builder
}

func (s *Service) GetBuilder(chain string) (*ethereum.TxBuilder, bool) {
	builder, ok := s.ethereumBuilders[chain]
	return builder, ok
}

func (s *Service) GetSolanaBuilder(chain string) (*solana.TxBuilder, bool) {
	builder, ok := s.solanaBuilders[chain]
	return builder, ok
}

func (s *Service) IsSolanaChain(chain string) bool {
	_, ok := s.solanaBuilders[chain]
	return ok
}

func (s *Service) SendTransactionEIP1559(ctx context.Context, chain string, to string, value *big.Int, ecdsaSigner *signer.ECDSASigner, speed FeeSpeed) (string, error) {
	builder, ok := s.ethereumBuilders[chain]
	if !ok {
		return "", fmt.Errorf("unsupported chain: %s", chain)
	}

	privateKey := ecdsaSigner.GetPrivateKey()

	_, rawTxHex, err := builder.BuildAndSignTxEIP1559(ctx, to, value, privateKey, ethereum.FeeSpeed(speed))
	if err != nil {
		return "", fmt.Errorf("failed to build/sign tx: %w", err)
	}

	txHash, err := builder.BroadcastTransaction(ctx, rawTxHex)
	if err != nil {
		return "", fmt.Errorf("failed to broadcast tx: %w", err)
	}

	return txHash, nil
}

func (s *Service) SendSolanaTransaction(ctx context.Context, chain string, from, to string, lamports uint64, solanaSigner *signer.SolanaSigner, confirm bool) (string, error) {
	builder, ok := s.solanaBuilders[chain]
	if !ok {
		return "", fmt.Errorf("unsupported chain: %s", chain)
	}

	unsignedTx, err := builder.BuildTransferTx(ctx, from, to, lamports)
	if err != nil {
		return "", fmt.Errorf("failed to build tx: %w", err)
	}

	signature, err := solanaSigner.SignTransaction(unsignedTx)
	if err != nil {
		return "", fmt.Errorf("failed to sign tx: %w", err)
	}

	signedTx, err := builder.AddSignature(unsignedTx, signature)
	if err != nil {
		return "", fmt.Errorf("failed to add signature: %w", err)
	}

	txHash, err := builder.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to broadcast tx: %w", err)
	}

	if confirm {
		if err := builder.ConfirmTransaction(ctx, txHash); err != nil {
			return txHash, fmt.Errorf("tx sent but confirmation failed: %w", err)
		}
	}

	return txHash, nil
}

func (s *Service) CheckBalanceSufficient(ctx context.Context, chain string, address string, value *big.Int, gasLimit uint64) (bool, *big.Int, error) {
	if s.IsSolanaChain(chain) {
		builder, ok := s.solanaBuilders[chain]
		if !ok {
			return false, nil, fmt.Errorf("unsupported chain: %s", chain)
		}
		lamports := value.Uint64()
		sufficient, required, err := builder.CheckBalanceSufficient(ctx, address, lamports)
		if err != nil {
			return false, nil, err
		}
		return sufficient, big.NewInt(int64(required)), nil
	}

	builder, ok := s.ethereumBuilders[chain]
	if !ok {
		return false, nil, fmt.Errorf("unsupported chain: %s", chain)
	}

	return builder.CheckBalanceSufficient(ctx, address, value, gasLimit)
}

func (s *Service) SendTransaction(ctx context.Context, chain string, to string, value *big.Int, ecdsaSigner *signer.ECDSASigner) (string, error) {
	builder, ok := s.ethereumBuilders[chain]
	if !ok {
		return "", fmt.Errorf("unsupported chain: %s", chain)
	}

	privateKey := ecdsaSigner.GetPrivateKey()

	_, rawTxHex, err := builder.BuildAndSignTx(ctx, to, value, privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to build/sign tx: %w", err)
	}

	txHash, err := builder.BroadcastTransaction(ctx, rawTxHex)
	if err != nil {
		return "", fmt.Errorf("failed to broadcast tx: %w", err)
	}

	return txHash, nil
}

func (s *Service) GetBalance(ctx context.Context, chain string, address string) (*big.Int, error) {
	if s.IsSolanaChain(chain) {
		builder, ok := s.solanaBuilders[chain]
		if !ok {
			return nil, fmt.Errorf("unsupported chain: %s", chain)
		}
		lamports, err := builder.GetBalance(ctx, address)
		if err != nil {
			return nil, err
		}
		return big.NewInt(int64(lamports)), nil
	}

	builder, ok := s.ethereumBuilders[chain]
	if !ok {
		return nil, fmt.Errorf("unsupported chain: %s", chain)
	}

	return builder.GetBalance(ctx, address)
}

func (s *Service) GetSolanaBalance(ctx context.Context, chain string, address string) (uint64, error) {
	builder, ok := s.solanaBuilders[chain]
	if !ok {
		return 0, fmt.Errorf("unsupported chain: %s", chain)
	}
	return builder.GetBalance(ctx, address)
}

func (s *Service) SendERC20Transaction(ctx context.Context, chain, tokenContract, to string, amount *big.Int, ecdsaSigner *signer.ECDSASigner, speed FeeSpeed) (string, error) {
	builder, ok := s.ethereumBuilders[chain]
	if !ok {
		return "", fmt.Errorf("unsupported chain: %s", chain)
	}

	privateKey := ecdsaSigner.GetPrivateKey()

	_, rawTxHex, err := builder.BuildAndSignERC20TransferTx(ctx, tokenContract, to, amount, privateKey, ethereum.FeeSpeed(speed))
	if err != nil {
		return "", fmt.Errorf("failed to build/sign erc20 tx: %w", err)
	}

	txHash, err := builder.BroadcastTransaction(ctx, rawTxHex)
	if err != nil {
		return "", fmt.Errorf("failed to broadcast tx: %w", err)
	}

	return txHash, nil
}

func (s *Service) SendSPLTransaction(ctx context.Context, chain, mint, to string, amount uint64, solanaSigner *signer.SolanaSigner, confirm bool) (string, error) {
	builder, ok := s.solanaBuilders[chain]
	if !ok {
		return "", fmt.Errorf("unsupported chain: %s", chain)
	}

	unsignedTx, err := builder.BuildSPLTransferTx(ctx, solanaSigner.Address(), to, mint, amount)
	if err != nil {
		return "", fmt.Errorf("failed to build spl tx: %w", err)
	}

	signature, err := solanaSigner.SignTransaction(unsignedTx)
	if err != nil {
		return "", fmt.Errorf("failed to sign tx: %w", err)
	}

	signedTx, err := builder.AddSignature(unsignedTx, signature)
	if err != nil {
		return "", fmt.Errorf("failed to add signature: %w", err)
	}

	txHash, err := builder.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to broadcast tx: %w", err)
	}

	if confirm {
		if err := builder.ConfirmTransaction(ctx, txHash); err != nil {
			return txHash, fmt.Errorf("tx sent but confirmation failed: %w", err)
		}
	}

	return txHash, nil
}

func (s *Service) GetERC20Balance(ctx context.Context, chain, tokenContract, address string) (*big.Int, error) {
	builder, ok := s.ethereumBuilders[chain]
	if !ok {
		return nil, fmt.Errorf("unsupported chain: %s", chain)
	}

	return builder.GetTokenBalance(ctx, tokenContract, address)
}

func (s *Service) GetSPLBalance(ctx context.Context, chain, owner, mint string) (uint64, error) {
	builder, ok := s.solanaBuilders[chain]
	if !ok {
		return 0, fmt.Errorf("unsupported chain: %s", chain)
	}

	return builder.GetTokenBalance(ctx, owner, mint)
}

func (s *Service) CheckTokenBalanceSufficient(ctx context.Context, chain, tokenContract, address string, tokenAmount *big.Int, gasLimit uint64) (bool, *big.Int, *big.Int, error) {
	if s.IsSolanaChain(chain) {
		builder, ok := s.solanaBuilders[chain]
		if !ok {
			return false, nil, nil, fmt.Errorf("unsupported chain: %s", chain)
		}
		sufficient, balance, err := builder.CheckSPLBalanceSufficient(ctx, address, tokenContract, tokenAmount.Uint64())
		return sufficient, big.NewInt(int64(balance)), nil, err
	}

	builder, ok := s.ethereumBuilders[chain]
	if !ok {
		return false, nil, nil, fmt.Errorf("unsupported chain: %s", chain)
	}

	return builder.CheckERC20BalanceSufficient(ctx, tokenContract, address, tokenAmount, gasLimit)
}

func CreateWallet() (*signer.ECDSASigner, string, error) {
	ecdsaSigner, err := signer.NewECDSASigner()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate key: %w", err)
	}

	address := ecdsaSigner.EthereumAddress()
	return ecdsaSigner, address, nil
}

func CreateSolanaWallet() (*signer.SolanaSigner, string, error) {
	solanaSigner, err := signer.NewSolanaSigner()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate key: %w", err)
	}

	address := solanaSigner.Address()
	return solanaSigner, address, nil
}

func DeriveAddressFromPrivateKey(privateKeyHex string) (string, error) {
	key, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}

	return crypto.PubkeyToAddress(key.PublicKey).Hex(), nil
}

func AddressFromPublicKey(pubKey []byte) (string, error) {
	if len(pubKey) != 65 {
		return "", fmt.Errorf("expected 65 bytes uncompressed public key")
	}

	ethKey := pubKey[1:]

	hash := crypto.Keccak256(ethKey)
	address := hash[12:]

	return fmt.Sprintf("0x%x", address), nil
}
