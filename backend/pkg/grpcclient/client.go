package grpcclient

import (
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AICoreClient struct {
	conn *grpc.ClientConn
}

func ConnectAIEngine(addr string) (*AICoreClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock(), grpc.WithTimeout(5*time.Second))
	if err != nil {
		log.Printf("Failed to connect to AI Engine via gRPC: %v", err)
		return nil, err
	}
	return &AICoreClient{conn: conn}, nil
}

func (c *AICoreClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
