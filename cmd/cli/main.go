package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yourorg/hsm/api/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	flagNodeAddr    = flag.String("node", "localhost:7001", "Node address")
	flagEnclaveAddr = flag.String("enclave", "localhost:7002", "Enclave address")
)

func main() {
	flag.Usage = func() {
		fmt.Println("HSM CLI - Distributed Key Custody")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  hsm-cli [options] <command> [arguments]")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  status              Check node status")
		fmt.Println("  dkg                 Start distributed key generation")
		fmt.Println("  sign <message>      Sign a message")
		fmt.Println("  verify <sig> <msg>  Verify a signature")
		fmt.Println("  key                 Show current public key")
		fmt.Println("  reshare             Refresh key shares")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	conn, err := grpc.Dial(*flagNodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to node: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := gen.NewNodeServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := flag.Arg(0)
	switch cmd {
	case "status":
		cmdStatus(ctx, client)
	case "dkg":
		cmdDKG(ctx, client)
	case "sign":
		cmdSign(ctx, client, flag.Args()[1:])
	case "verify":
		cmdVerify(flag.Args()[1:], *flagEnclaveAddr)
	case "key":
		cmdKey(ctx, client)
	case "reshare":
		cmdReshare(ctx, client)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

func cmdStatus(ctx context.Context, client gen.NodeServiceClient) {
	resp, err := client.Handshake(ctx, &gen.HandshakeRequest{
		NodeId:    0,
		ClusterId: "unknown",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Node Status:")
	fmt.Printf("  Node ID:     %d\n", resp.NodeId)
	fmt.Printf("  Cluster ID:  %s\n", resp.ClusterId)
	fmt.Printf("  Status:      %v\n", map[bool]string{true: "Ready", false: "Not Ready"}[resp.Accepted])
}

func cmdDKG(ctx context.Context, client gen.NodeServiceClient) {
	fmt.Println("Starting DKG...")

	resp, err := client.DKGMessage(ctx, &gen.NodeMessage{
		MessageType: "trigger_dkg",
		FromNode:    0,
		Payload:     []byte("start"),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(resp.Payload) > 0 {
		fmt.Printf("Public Key: %s\n", hex.EncodeToString(resp.Payload))
	}
	fmt.Println("DKG completed successfully")
}

func cmdSign(ctx context.Context, client gen.NodeServiceClient, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: hsm-cli sign <message>")
		os.Exit(1)
	}

	message := args[0]
	fmt.Printf("Signing message: %s\n", message)

	resp, err := client.SignMessage(ctx, &gen.NodeMessage{
		MessageType: "trigger_sign",
		FromNode:    0,
		Payload:     []byte(message),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(resp.Payload) > 0 {
		fmt.Printf("Signature: %s\n", hex.EncodeToString(resp.Payload))
	}
	fmt.Println("Signing completed")
}

func cmdVerify(args []string, enclaveAddr string) {
	if len(args) < 2 {
		fmt.Println("Usage: hsm-cli verify <signature_hex> <message>")
		os.Exit(1)
	}

	sigHex := args[0]
	message := args[1]

	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid signature hex: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Verifying signature: %s\n", sigHex)
	fmt.Printf("Message: %s\n", message)

	// Call enclave directly via HTTP
	enclaveURL := "http://" + enclaveAddr + "/verify"

	var resp struct {
		Valid bool   `json:"valid"`
		Error string `json:"error"`
	}

	// Use http.Post for simplicity
	jsonBody, _ := json.Marshal(map[string]interface{}{
		"signature": sig,
		"message":   []byte(message),
	})

	httpResp, err := http.Post(enclaveURL, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Verify request failed: %v\n", err)
		os.Exit(1)
	}
	defer httpResp.Body.Close()

	json.NewDecoder(httpResp.Body).Decode(&resp)

	if resp.Valid {
		fmt.Println("Result: VALID ✓")
	} else {
		fmt.Printf("Result: INVALID ✗\n")
		if resp.Error != "" {
			fmt.Printf("Error: %s\n", resp.Error)
		}
	}
}

func cmdKey(ctx context.Context, client gen.NodeServiceClient) {
	// Get status which includes public key info
	resp, err := client.Handshake(ctx, &gen.HandshakeRequest{
		NodeId:    0,
		ClusterId: "unknown",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(resp.PublicKey) > 0 {
		fmt.Printf("Public Key: %s\n", hex.EncodeToString(resp.PublicKey))
	} else {
		fmt.Println("No public key available. Run 'dkg' first.")
	}
}

func cmdReshare(ctx context.Context, client gen.NodeServiceClient) {
	fmt.Println("Starting key resharing...")

	resp, err := client.DKGMessage(ctx, &gen.NodeMessage{
		MessageType: "trigger_reshare",
		FromNode:    0,
		Payload:     []byte("start"),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Resharing completed successfully")
	_ = resp
}

func parseSigners(s string) []uint32 {
	if s == "" {
		return []uint32{1}
	}
	var signers []uint32
	for _, part := range strings.Split(s, ",") {
		var id uint32
		fmt.Sscanf(part, "%d", &id)
		signers = append(signers, id)
	}
	return signers
}
