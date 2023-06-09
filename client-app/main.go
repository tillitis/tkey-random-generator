// Copyright (C) 2022, 2023 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"bytes"
	"crypto/ed25519"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	"github.com/tillitis/tkeyclient"
	"golang.org/x/crypto/blake2s"
)

// nolint:typecheck // Avoid lint error when the embedding file is missing.
// Makefile copies the built app here ./app.bin
//
//go:embed app.bin
var appBinary []byte

const (
	wantFWName0  = "tk1 "
	wantFWName1  = "mkdf"
	wantAppName0 = "tk1 "
	wantAppName1 = "rand"
)

var le = log.New(os.Stderr, "", 0)

func main() {
	var devPath, filePath string
	var speed, genBytes int
	var helpOnly, sig, toFile bool
	pflag.CommandLine.SortFlags = false
	pflag.StringVarP(&devPath, "port", "p", "",
		"Set serial port device `PATH`. If this is not passed, auto-detection will be attempted.")
	pflag.IntVar(&speed, "speed", tkeyclient.SerialSpeed,
		"Set serial port speed in `BPS` (bits per second).")
	pflag.IntVarP(&genBytes, "bytes", "b", 0,
		"Fetch `COUNT` number of random bytes.")
	pflag.BoolVarP(&sig, "signature", "s", false, "Get the signature of the generated random data.")
	pflag.StringVarP(&filePath, "file", "f", "",
		"Output random data as binary to `FILE`.")
	pflag.BoolVarP(&helpOnly, "help", "h", false, "Output this help.")
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, `tkey-random-generator is a client app used to fetch random numbers
from the TRNG on the Tillitis TKey. This program embeds the random generator-app binary,
which it loads onto the TKey and starts.

Usage:

%s`,
			pflag.CommandLine.FlagUsagesWrapped(80))
	}
	pflag.Parse()

	if helpOnly {
		pflag.Usage()
		os.Exit(0)
	}

	if genBytes == 0 {
		le.Printf("Please set number of bytes with --bytes\n")
		pflag.Usage()
		os.Exit(2)
	}

	if devPath == "" {
		var err error
		devPath, err = tkeyclient.DetectSerialPort(true)
		if err != nil {
			os.Exit(1)
		}
	}
	var file *os.File
	var fileErr error
	if filePath != "" {
		toFile = true
		file, fileErr = os.Create(filePath)
		if fileErr != nil {
			le.Printf("Could not create file %s: %v\n", filePath, fileErr)
			os.Exit(1)
		}
	}

	tkeyclient.SilenceLogging()

	tk := tkeyclient.New()
	le.Printf("Connecting to device on serial port %s...\n", devPath)
	if err := tk.Connect(devPath, tkeyclient.WithSpeed(speed)); err != nil {
		le.Printf("Could not open %s: %v\n", devPath, err)
		os.Exit(1)
	}

	randomGen := New(tk)
	exit := func(code int) {
		if err := randomGen.Close(); err != nil {
			le.Printf("%v\n", err)
		}
		os.Exit(code)
	}
	handleSignals(func() { exit(1) }, os.Interrupt, syscall.SIGTERM)

	if isFirmwareMode(tk) {
		le.Printf("Device is in firmware mode. Loading app...\n")
		if err := tk.LoadApp(appBinary, []byte{}); err != nil {
			le.Printf("LoadApp failed: %v", err)
			exit(1)
		}
	}

	if !isWantedApp(randomGen) {
		fmt.Printf("The TKey may already be running an app, but not the expected random-app.\n" +
			"Please unplug and plug it in again.\n")
		exit(1)
	}

	if !toFile {
		le.Printf("Random data follows on stdout...\n\n")
	} else {
		le.Printf("Writing random data to: %s\n", filePath)
	}

	var totRandom []byte

	left := genBytes
	for {
		get := left
		if get > RandomPayloadMaxBytes {
			get = RandomPayloadMaxBytes
		}
		random, err := randomGen.GetRandom(get)
		if err != nil {
			le.Printf("GetRandom failed: %v\n", err)
			exit(1)
		}
		totRandom = append(totRandom, random...)

		if toFile {
			_, err := file.Write(random)
			if err != nil {
				le.Printf("Error could not write to file %v\n", err)
				exit(1)
			}
		} else {
			fmt.Printf("%x", random)
		}

		if left > len(random) {

			left -= len(random)
			continue
		}
		break
	}

	fmt.Printf("\n\n")

	// Always fetch the signature and hash to re-init the hash on the TKey
	signature, hash, err := randomGen.GetSignature()
	if err != nil {
		le.Printf("GetSig failed: %v\n", err)
		exit(1)
	}

	// Only print and verify if asked
	if sig {
		pubkey, err := randomGen.GetPubkey()
		if err != nil {
			le.Printf("GetPubkey failed: %v\n", err)
			exit(1)
		}

		fmt.Printf("Public key: %x\n", pubkey)
		fmt.Printf("Signature: %x\n", signature)
		fmt.Printf("Hash: %x\n", hash)

		le.Print(("\nVerifying signature ... "))
		if !ed25519.Verify(pubkey, hash, signature) {
			le.Printf("signature FAILED verification.\n")
			// Don't exit, let's calculate hash
		} else {
			le.Printf("signature verified.\n")
		}

		localHash := blake2s.Sum256(totRandom)

		le.Printf("\nVerifying hash ... ")

		if !bytes.Equal(hash, localHash[:]) {
			le.Printf("hash FAILED verification.\n")
			exit(1)
		}
		le.Printf("hash verified.\n")
	}

	file.Close()
	exit(0)
}

func handleSignals(action func(), sig ...os.Signal) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sig...)
	go func() {
		for {
			<-ch
			action()
		}
	}()
}

func isFirmwareMode(tk *tkeyclient.TillitisKey) bool {
	nameVer, err := tk.GetNameVersion()
	if err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, tkeyclient.ErrResponseStatusNotOK) {
			le.Printf("GetNameVersion failed: %s\n", err)
		}
		return false
	}
	// not caring about nameVer.Version
	return nameVer.Name0 == wantFWName0 &&
		nameVer.Name1 == wantFWName1
}

func isWantedApp(randomGen RandomGen) bool {
	nameVer, err := randomGen.GetAppNameVersion()
	if err != nil {
		if !errors.Is(err, io.EOF) {
			le.Printf("GetAppNameVersion: %s\n", err)
		}
		return false
	}
	// not caring about nameVer.Version
	return nameVer.Name0 == wantAppName0 &&
		nameVer.Name1 == wantAppName1
}
