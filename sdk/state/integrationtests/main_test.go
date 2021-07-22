package integrationtests

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stellar/go/clients/horizonclient"
)

const horizonURL = "http://localhost:8000"

var (
	networkPassphrase string
	client            *horizonclient.Client
)

func TestMain(m *testing.M) {
	if os.Getenv("INTEGRATION_TESTS") == "" {
		fmt.Fprintln(os.Stderr, "SKIP")
		os.Exit(0)
	}

	client = &horizonclient.Client{
		HorizonURL: horizonURL,
		HTTP: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
	networkDetails, err := client.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	networkPassphrase = networkDetails.NetworkPassphrase

	os.Exit(m.Run())
}
