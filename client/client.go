package client

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/icellan/go-mempool/chainhash"
	"github.com/icellan/go-mempool/wire"
	"github.com/libsv/go-bt/v2"
)

type Client struct {
	Connections            []*connection
	callbacksOnTransaction []func([]byte)
	callbacksOnBlock       []func([]byte)
	callbacksOnError       []func([]byte)
	// TODO change this to a cache with a TTL
	transactionQueue map[string]*bt.Tx
}

type connection struct {
	address    string
	connection *net.Conn
}

func New(addresses []string) (*Client, error) {
	c := &Client{
		Connections:      make([]*connection, 0),
		transactionQueue: make(map[string]*bt.Tx, 0),
	}

	for _, address := range addresses {
		conn, err := net.Dial("tcp", address)
		if err != nil {
			log.Println(err)
			continue
		} else {
			cc := &connection{
				address:    address,
				connection: &conn,
			}

			go func() {
				for {
					// handle incoming requests on the connection
					// passing the client, for the callbacks
					cc.HandleRequests(c)
				}
			}()

			if err = cc.SendHandshake(); err != nil {
				log.Println(err)
			} else {
				c.Connections = append(c.Connections, cc)
			}
		}
	}

	if len(c.Connections) == 0 {
		return nil, fmt.Errorf("[CLIENT] Could not connect to any nodes: %s", addresses)
	}

	return c, nil
}

func (c *Client) OnTransaction(f func([]byte)) {
	c.callbacksOnTransaction = append(c.callbacksOnTransaction, f)
}

func (c *Client) OnBlock(f func([]byte)) {
	c.callbacksOnBlock = append(c.callbacksOnBlock, f)
}

func (c *Client) OnError(f func([]byte)) {
	c.callbacksOnError = append(c.callbacksOnError, f)
}

func (c *Client) SendTx(txBytes []byte) error {
	// basic sanity check on the tx
	tx, err := bt.NewTxFromBytes(txBytes)
	if err != nil {
		return err
	}

	var hash *chainhash.Hash
	hash, err = chainhash.NewHashFromStr(tx.TxID())
	if err != nil {
		return err
	}

	msgInv := wire.NewMsgInv()
	_ = msgInv.AddInvVect(wire.NewInvVect(wire.InvTypeTx, hash))
	if err = c.WriteMessage(msgInv); err != nil {
		return err
	}

	// store the transaction in our own "mempool" for when it is requested by the peer node
	c.transactionQueue[hash.String()] = tx

	return nil
}

func (c *Client) GetTx(txID string) ([]byte, error) {

	hash, err := chainhash.NewHashFromStr(txID)
	if err != nil {
		return nil, err
	}

	if tx, ok := c.transactionQueue[hash.String()]; ok {
		return tx.Bytes(), nil
	}

	// request the transaction from a peer
	msg := wire.NewMsgGetData()
	if err = msg.AddInvVect(wire.NewInvVect(wire.InvTypeTx, hash)); err != nil {
		return nil, err
	}

	if err = c.WriteMessage(msg); err != nil {
		return nil, err
	}

	// wait for the message for 3 seconds
	for i := 0; i < 3; i++ {
		if tx, ok := c.transactionQueue[hash.String()]; ok {
			return tx.Bytes(), nil
		}
		time.Sleep(1 * time.Second)
	}

	return nil, fmt.Errorf("[CLIENT] ERROR could not get transaction from peers: %s", hash.String())
}

func (c *Client) WriteMessage(msg wire.Message) error {
	// write the message out to all connected peers
	var msgErrors []error
	for _, conn := range c.Connections {
		if err := conn.WriteMessage(msg); err != nil {
			msgErrors = append(msgErrors, err)
		}
	}

	if len(msgErrors) > 0 {
		// TODO how to handle multiple errors
		return msgErrors[0]
	}

	return nil
}

func (conn *connection) SendHandshake() error {
	msg := versionMessage()
	if err := conn.WriteMessage(msg); err != nil {
		return err
	}

	return nil
}

func (conn *connection) WriteMessage(msg wire.Message) error {
	return wire.WriteMessage(*conn.connection, msg, wire.ProtocolVersion, wire.TestNet)
}

// HandleRequests handles incoming requests.
func (conn *connection) HandleRequests(c *Client) {
	msg, b, err := wire.ReadMessage(*conn.connection, wire.ProtocolVersion, wire.TestNet)
	if err != nil {
		log.Printf("[CLIENT] READ ERROR: %v\n", err)
		if errors.Is(err, io.EOF) {
			// try to reconnect to the node
			var newConn net.Conn
			log.Printf("[CLIENT] Lost connection to %s, retrying", conn.address)
			newConn, err = net.Dial("tcp", conn.address)
			if err != nil {
				log.Printf("[CLIENT] ERROR: %v\n", err)
				time.Sleep(2 * time.Second)
			} else {
				_ = (*conn.connection).Close()
				conn.connection = &newConn
				if err = conn.SendHandshake(); err != nil {
					log.Printf("[CLIENT] ERROR: %v\n", err)
				}
			}
		}
		return
	}

	switch msg.Command() {
	case wire.CmdVersion:
		msg = wire.NewMsgVerAck()

		err = wire.WriteMessage(*conn.connection, msg, wire.ProtocolVersion, wire.TestNet)
		if err != nil {
			log.Printf("[CLIENT] ERROR: %v\n", err) // Log and handle the error
			return
		}
	case wire.CmdPing:
		pingMsg := msg.(*wire.MsgPing)
		msg = wire.NewMsgPong(pingMsg.Nonce)

		err = wire.WriteMessage(*conn.connection, msg, wire.ProtocolVersion, wire.TestNet)
		if err != nil {
			log.Printf("[CLIENT] ERROR: %v\n", err) // Log and handle the error
			return
		}
	case wire.CmdInv:
		invMsg := msg.(*wire.MsgInv)
		for _, inv := range invMsg.InvList {
			switch inv.Type {
			case wire.InvTypeTx:
				for _, callback := range c.callbacksOnTransaction {
					callback(inv.Hash.CloneBytes())
				}
			case wire.InvTypeBlock:
				for _, callback := range c.callbacksOnBlock {
					callback(inv.Hash.CloneBytes())
				}
			case wire.InvTypeFilteredBlock:
				for _, callback := range c.callbacksOnBlock {
					callback(inv.Hash.CloneBytes())
				}
			case wire.InvTypeError:
				for _, callback := range c.callbacksOnBlock {
					callback(inv.Hash.CloneBytes())
				}
			}
		}

	case wire.CmdReject:
		rejMsg := msg.(*wire.MsgReject)
		log.Printf("[CLIENT] Reject received: %s: %s [%d]\n", rejMsg.Reason, rejMsg.Hash, rejMsg.Code)

	case wire.CmdNotFound:
		notFoundMsg := msg.(*wire.MsgNotFound)
		log.Printf("[CLIENT] Not found received: %v\n", notFoundMsg.InvList)

	case wire.CmdGetData:
		dataMsg := msg.(*wire.MsgGetData)
		log.Printf("[CLIENT] Received %s - %v\n", msg.Command(), dataMsg)
		for _, inv := range dataMsg.InvList {
			if inv.Type == wire.InvTypeTx {
				// send the tx back
				txID := inv.Hash.String()
				if tx, ok := c.transactionQueue[txID]; ok {
					txMsg := &wire.MsgTx{
						Version:  int32(tx.Version),
						LockTime: tx.LockTime,
					}

					for _, input := range tx.Inputs {
						h, _ := chainhash.NewHash(input.PreviousTxID())
						txMsg.AddTxIn(wire.NewTxIn(&wire.OutPoint{
							Hash:  *h,
							Index: input.PreviousTxOutIndex,
						}, *input.UnlockingScript))
					}

					for _, output := range tx.Outputs {
						txMsg.AddTxOut(wire.NewTxOut(int64(output.Satoshis), *output.LockingScript))
					}

					if err = conn.WriteMessage(txMsg); err != nil {
						log.Printf("[CLIENT] ERROR sending tx %s - %s\n", tx.TxID(), err.Error())
					} else {
						// TODO do not delete here, use cache with a TTL
						delete(c.transactionQueue, txID)
					}
				}
			}
		}

	default:
		log.Printf("[CLIENT] Received %s - %s\n", msg.Command(), string(b))
	}
}

func versionMessage() *wire.MsgVersion {
	// Create version message data.
	lastBlock := int32(0)
	tcpAddrMe := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9999}
	me := wire.NewNetAddress(tcpAddrMe, wire.SFNodeNetwork)

	tcpAddrYou := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 18333}
	you := wire.NewNetAddress(tcpAddrYou, wire.SFNodeNetwork)

	nonce, err := wire.RandomUint64()

	if err != nil {
		log.Printf("[CLIENT] RandomUint64: error generating nonce: %v\n", err)
	}

	// Ensure we get the correct data back out.
	msg := wire.NewMsgVersion(me, you, nonce, lastBlock)

	return msg
}

func ReverseHex(hexString string) string {
	bytes, _ := hex.DecodeString(hexString)
	reversed := bt.ReverseBytes(bytes)
	return hex.EncodeToString(reversed)
}
