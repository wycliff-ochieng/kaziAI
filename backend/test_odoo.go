package main

import (
	"context"
	"fmt"
	"log"

	"erpagent-backend/internal/adapters/odoo"
	"erpagent-backend/internal/domain"
)

func main() {
	adapter := odoo.NewOdooAdapter("http://192.168.128.6:8069", "kazi", "admin", "admin")
	
	filter := domain.CustomerFilter{Search: "Eslam and add one shot"}
	custs, err := adapter.GetCustomers(context.Background(), filter)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	
	fmt.Printf("Total customers returned: %d\n", len(custs))
	if len(custs) > 0 {
		fmt.Printf("First customer: %s (ID: %s)\n", custs[0].Name, custs[0].ID)
	}
}
