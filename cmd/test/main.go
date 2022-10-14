package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/icellan/go-mempool/client"
)

var (
	// txID  = "c1ef62014e07692b8e1a06aa7b4def87b00af6059280599399ed95f41cbf7617"
	tx, _ = hex.DecodeString("0200000001203f7259e997225847ffb9b2aeee706008fc398f354db797e762a676f998272a0000000049483045022100d65ae0107cf9e98eb3156da644fab428061137e7fc798cd9751d1b816e4e0c58022078fe1ee5f25e0393742172969c6dbbf3db1e92ca0f784a25e08069efce39f3c441feffffff02804c6d29010000001976a9149d301cf4920b35ecfbeb778ef58d47f761ea8aba88ac80969800000000001976a914a620a6c640feb3d2f18757d1b2e4438719fba84c88ac59000000")
)

func main() {
	mempoolClient, err := client.New([]string{"127.0.0.1:18333", "127.0.0.1:18333"})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	mempoolClient.OnTransaction(func(txID []byte) {
		fmt.Printf("NEW TRANSACTION NOTIFICATION: %s\n", hex.EncodeToString(txID))
	})
	mempoolClient.OnBlock(func(blockID []byte) {
		fmt.Printf("NEW BLOCK NOTIFICATION: %s\n", hex.EncodeToString(blockID))
	})

	// we need some time for the initial version message to be sent
	time.Sleep(1 * time.Second)

	// send a new transaction out into the mempool
	if err = mempoolClient.SendTx(tx); err != nil {
		fmt.Printf("ERROR sending transaction to mempool: %x\n", err)
	}

	// get a transaction from the peer nodes
	if tx, err = mempoolClient.GetTx("1610a35b10358013db4837c411412d037252c82e739f70c9491f02a05e8db4ee"); err != nil {
		fmt.Printf("ERROR getting transaction from peer: %v\n", err)
	}

	ch := make(chan bool)
	<-ch
}
