package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/yourorg/hsm/api/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	flagNodeAddr = flag.String("node", "localhost:7001", "Node address")
	flagAction   = flag.String("action", "", "Action: dkg, sign, status")
	flagMessage  = flag.String("message", "", "Message to sign (hex)")
	flagSigners  = flag.String("signers", "", "Comma-separated list of signer node IDs")
)

func main() {
	flag.Parse()

	if *flagAction == "" {
		fmt.Println("Usage: operator -node <addr> -action <dkg|sign|status> [-message <hex>] [-signers <ids>]")
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
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	switch *flagAction {
	case "dkg":
		handleDKG(ctx, client)
	case "sign":
		handleSign(ctx, client)
	case "status":
		handleStatus(ctx, client)
	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %s\n", *flagAction)
		os.Exit(1)
	}
}

func handleDKG(ctx context.Context, client gen.NodeServiceClient) {
	fmt.Println("Triggering DKG...")

	resp, err := client.DKGMessage(ctx, &gen.NodeMessage{
		MessageType: "trigger_dkg",
		FromNode:    0,
		Payload:     []byte("start"),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "DKG failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("DKG response: %s\n", resp.MessageType)
	if len(resp.Payload) > 0 {
		fmt.Printf("Public key: %x\n", resp.Payload)
	}
	fmt.Println("DKG completed successfully")
}

func handleSign(ctx context.Context, client gen.NodeServiceClient) {
	if *flagMessage == "" || *flagSigners == "" {
		fmt.Println("sign action requires -message and -signers")
		os.Exit(1)
	}

	var signers []uint32
	fmt.Sscanf(*flagSigners, "%d", &signers)

	fmt.Printf("Signing message: %s\n", *flagMessage)
	fmt.Printf("Signers: %v\n", signers)

	resp, err := client.SignMessage(ctx, &gen.NodeMessage{
		MessageType: "trigger_sign",
		FromNode:    0,
		Payload:     []byte(*flagMessage),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Signing failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Sign response: %s\n", resp.MessageType)
	if len(resp.Payload) > 0 {
		fmt.Printf("Signature: %x\n", resp.Payload)
	}
	fmt.Println("Signing completed successfully")
}

func handleStatus(ctx context.Context, client gen.NodeServiceClient) {
	fmt.Println("Getting node status...")

	resp, err := client.Handshake(ctx, &gen.HandshakeRequest{
		NodeId:    0,
		ClusterId: "unknown",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Status check failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Node ID: %d\n", resp.NodeId)
	fmt.Printf("Cluster ID: %s\n", resp.ClusterId)
	fmt.Printf("Accepted: %v\n", resp.Accepted)
}
