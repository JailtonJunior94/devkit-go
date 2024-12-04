package main

import (
	"github.com/JailtonJunior94/devkit-go/examples/order/api"
	"github.com/JailtonJunior94/devkit-go/examples/order/consumer"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "order",
		Short: "Order",
	}

	server := &cobra.Command{
		Use:   "api",
		Short: "Order API",
		Run: func(cmd *cobra.Command, args []string) {
			api.NewApiServer().Run()
		},
	}

	consumers := &cobra.Command{
		Use:   "consumers",
		Short: "Order Consumers",
		Run: func(cmd *cobra.Command, args []string) {
			consumer.NewConsumer().Run()
		},
	}

	root.AddCommand(server, consumers)
	root.Execute()
}
