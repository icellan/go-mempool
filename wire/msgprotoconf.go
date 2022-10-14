// Copyright (c) 2013-2015 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"io"
)

// MsgProtoconf implements the Message interface and represents a bitcoin protoconf
// message.
type MsgProtoconf struct {
	// Unique value associated with message that is used to identify
	// specific ping message.
	Nonce uint64
}

// Bsvdecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgProtoconf) Bsvdecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	// There was no nonce for BIP0031Version and earlier.
	// NOTE: > is not a mistake here.  The BIP0031 was defined as AFTER
	// the version unlike most others.
	if pver > BIP0031Version {
		err := readElement(r, &msg.Nonce)
		if err != nil {
			return err
		}
	}

	return nil
}

// BsvEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgProtoconf) BsvEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	// There was no nonce for BIP0031Version and earlier.
	// NOTE: > is not a mistake here.  The BIP0031 was defined as AFTER
	// the version unlike most others.
	if pver > BIP0031Version {
		err := writeElement(w, msg.Nonce)
		if err != nil {
			return err
		}
	}

	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgProtoconf) Command() string {
	return CmdProtoconf
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgProtoconf) MaxPayloadLength(pver uint32) uint32 {
	plen := uint32(0)
	// There was no nonce for BIP0031Version and earlier.
	// NOTE: > is not a mistake here.  The BIP0031 was defined as AFTER
	// the version unlike most others.
	if pver > BIP0031Version {
		// Nonce 8 bytes.
		plen += 8
	}

	return plen
}

// NewMsgProtoconf returns a new bitcoin protoconf message that conforms to the Message
// interface.  See MsgProtoconf for details.
func NewMsgProtoconf(nonce uint64) *MsgProtoconf {
	return &MsgProtoconf{}
}
