package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lovi-cloud/ursa"
)

func main() {
	err := ursa.Run(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
