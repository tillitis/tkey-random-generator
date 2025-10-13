// Copyright (C) 2022, 2023 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

package generator

import (
	_ "embed"
	"errors"
	"fmt"
	"io"

	"github.com/tillitis/tkeyclient"
)

const (
	wantFWName0  = "tk1 "
	wantFWName1  = "mkdf"
	wantAppName0 = "tk1 "
	wantAppName1 = "rand"
)

// nolint:typecheck // Avoid lint error when the embedding file is missing.
// Makefile copies the built app here ./app.bin
//
//go:embed app.bin
var appBinary []byte

var (
	cmdGetNameVersion = appCmd{0x01, "cmdGetNameVersion", tkeyclient.CmdLen1}
	rspGetNameVersion = appCmd{0x02, "rspGetNameVersion", tkeyclient.CmdLen32}
	cmdGetRandom      = appCmd{0x03, "cmdGetRandom", tkeyclient.CmdLen4}
	rspGetRandom      = appCmd{0x04, "rspGetRandom", tkeyclient.CmdLen128}
	cmdGetPubkey      = appCmd{0x05, "cmdGetPubkey", tkeyclient.CmdLen1}
	rspGetPubkey      = appCmd{0x06, "rspGetPubkey", tkeyclient.CmdLen128}
	cmdGetSig         = appCmd{0x07, "cmdGetSig", tkeyclient.CmdLen1}
	rspCmdSig         = appCmd{0x08, "rspCmdSig", tkeyclient.CmdLen128}
)

// cmdlen - (responsecode + status)
var RandomPayloadMaxBytes = rspGetRandom.CmdLen().Bytelen() - (1 + 1)

type appCmd struct {
	code   byte
	name   string
	cmdLen tkeyclient.CmdLen
}

func (c appCmd) Code() byte {
	return c.code
}

func (c appCmd) CmdLen() tkeyclient.CmdLen {
	return c.cmdLen
}

func (c appCmd) Endpoint() tkeyclient.Endpoint {
	return tkeyclient.DestApp
}

func (c appCmd) String() string {
	return c.name
}

type RandomGen struct {
	tk *tkeyclient.TillitisKey // A connection to a TKey
}

func (r RandomGen) isFirmwareMode() (bool, error) {
	nameVer, err := r.tk.GetNameVersion()
	if err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, tkeyclient.ErrResponseStatusNotOK) {
			return false, fmt.Errorf("%w", err)
		}

		return false, nil
	}

	// not caring about nameVer.Version
	return nameVer.Name0 == wantFWName0 &&
		nameVer.Name1 == wantFWName1, nil
}

func (r RandomGen) isWantedApp() (bool, error) {
	nameVer, err := r.GetAppNameVersion()
	if err != nil {
		if !errors.Is(err, io.EOF) {
			return false, fmt.Errorf("%w", err)
		}
		return false, nil
	}
	// not caring about nameVer.Version
	return nameVer.Name0 == wantAppName0 &&
		nameVer.Name1 == wantAppName1, nil
}

func (r RandomGen) loadApp(USS []byte) error {
	isFirmware, err := r.isFirmwareMode()
	if err != nil {
		return err
	}

	if isFirmware {
		if err := r.tk.LoadApp(appBinary, USS); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	rightApp, err := r.isWantedApp()
	if err != nil {
		return err
	}

	if !rightApp {
		return fmt.Errorf("wrong app running")
	}

	return nil
}

// New loads the random-generator device app and provides functions to
// communicate with. You're expected to pass an existing TKey connection to
// it, so use it like this:
//
//	tk := tkeyclient.New()
//	err := tk.Connect(port)
//	randomGen := New(tk)
func New(tk *tkeyclient.TillitisKey) (RandomGen, error) {
	randomGen := RandomGen{
		tk: tk,
	}

	err := randomGen.loadApp([]byte{})
	if err != nil {
		return RandomGen{}, fmt.Errorf("couldn't load app: %w", err)
	}

	return randomGen, nil
}

// Close closes the connection to the TKey
func (s RandomGen) Close() error {
	if err := s.tk.Close(); err != nil {
		return fmt.Errorf("tk.Close: %w", err)
	}
	return nil
}

// GetAppNameVersion gets the name and version of the running app in
// the same style as the stick itself.
func (s RandomGen) GetAppNameVersion() (*tkeyclient.NameVersion, error) {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdGetNameVersion, id)
	if err != nil {
		return nil, fmt.Errorf("NewFrameBuf: %w", err)
	}

	tkeyclient.Dump("GetAppNameVersion tx", tx)
	if err = s.tk.Write(tx); err != nil {
		return nil, fmt.Errorf("Write: %w", err)
	}

	defer s.tk.SetReadTimeoutNoErr(0)
	s.tk.SetReadTimeoutNoErr(2)

	rx, _, err := s.tk.ReadFrame(rspGetNameVersion, id)
	if err != nil {
		return nil, fmt.Errorf("ReadFrame: %w", err)
	}

	nameVer := &tkeyclient.NameVersion{}
	nameVer.Unpack(rx[2:])

	return nameVer, nil
}

// GetRandom fetches random data.
func (s RandomGen) GetRandom(bytes int) ([]byte, error) {
	if bytes < 1 || bytes > RandomPayloadMaxBytes {
		return nil, fmt.Errorf("number of bytes is not in [1,%d]", RandomPayloadMaxBytes)
	}

	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdGetRandom, id)
	if err != nil {
		return nil, fmt.Errorf("NewFrameBuf: %w", err)
	}

	tx[2] = byte(bytes)
	tkeyclient.Dump("GetRandom tx", tx)
	if err = s.tk.Write(tx); err != nil {
		return nil, fmt.Errorf("Write: %w", err)
	}

	rx, _, err := s.tk.ReadFrame(rspGetRandom, id)
	tkeyclient.Dump("GetRandom rx", rx)
	if err != nil {
		return nil, fmt.Errorf("ReadFrame: %w", err)
	}

	if rx[2] != tkeyclient.StatusOK {
		return nil, fmt.Errorf("GetRandom NOK")
	}

	ret := RandomPayloadMaxBytes
	if ret > bytes {
		ret = bytes
	}
	// Skipping frame header, app header, and status
	return rx[3 : 3+ret], nil
}

// GetPubkey fetches the public key of the signer.
func (s RandomGen) GetPubkey() ([]byte, error) {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdGetPubkey, id)
	if err != nil {
		return nil, fmt.Errorf("NewFrameBuf: %w", err)
	}

	tkeyclient.Dump("GetPubkey tx", tx)
	if err = s.tk.Write(tx); err != nil {
		return nil, fmt.Errorf("Write: %w", err)
	}

	rx, _, err := s.tk.ReadFrame(rspGetPubkey, id)
	tkeyclient.Dump("GetPubKey rx", rx)
	if err != nil {
		return nil, fmt.Errorf("ReadFrame: %w", err)
	}

	// Skip frame header & app header, returning size of ed25519 pubkey
	return rx[2 : 2+32], nil
}

// GetSignature returns both the signature and the calculated hash
// over the generated random data.
func (s RandomGen) GetSignature() ([]byte, []byte, error) {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdGetSig, id)
	if err != nil {
		return nil, nil, fmt.Errorf("NewFrameBuf: %w", err)
	}

	tkeyclient.Dump("GetSig tx", tx)
	if err = s.tk.Write(tx); err != nil {
		return nil, nil, fmt.Errorf("Write: %w", err)
	}

	rx, _, err := s.tk.ReadFrame(rspCmdSig, id)
	tkeyclient.Dump("GetSig rx", rx)
	if err != nil {
		return nil, nil, fmt.Errorf("ReadFrame: %w", err)
	}

	// Skipping frame header & app header
	return rx[3 : 3+64], rx[3+64 : 3+64+32], nil
}
