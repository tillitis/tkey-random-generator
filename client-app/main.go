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
	"github.com/tillitis/tkeyutil"
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
	var fileUSS, devPath, filePath string
	var speed, genBytes int
	var enterUSS, helpOnly, sig bool
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
	pflag.BoolVar(&enterUSS, "uss", false,
		"Enable typing of a phrase to be hashed as the User Supplied Secret. The USS is loaded onto the TKey along with the app itself. A different USS results in different Compound Device Identifier, different start of the random sequence, and another key pair used for signing.")
	pflag.StringVar(&fileUSS, "uss-file", "",
		"Read `FILE` and hash its contents as the USS. Use '-' (dash) to read from stdin. The full contents are hashed unmodified (e.g. newlines are not stripped).")

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

	if enterUSS && fileUSS != "" {
		le.Printf("Pass only one of --uss or --uss-file.\n\n")
		pflag.Usage()
		os.Exit(2)
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

	err := loadApp(tk, enterUSS, fileUSS)
	if err != nil {
		le.Printf("Couldn't load app: %v", err)
		exit(1)
	}

	if !isWantedApp(randomGen) {
		fmt.Printf("The TKey may already be running an app, but not the expected random-app.\n" +
			"Please unplug and plug it in again.\n")
		exit(1)
	}

	totRandom, err := genRandomData(randomGen, genBytes, filePath)
	if err != nil {
		le.Printf("Couldn't generate random data: %v\n", err)
		exit(1)
	}

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

func loadApp(tk *tkeyclient.TillitisKey, enterUSS bool, fileUSS string) error {
	if isFirmwareMode(tk) {
		var secret []byte
		var err error

		if enterUSS {
			secret, err = tkeyutil.InputUSS()
			if err != nil {
				return fmt.Errorf("InputUSS: %w", err)
			}
		}
		if fileUSS != "" {
			secret, err = tkeyutil.ReadUSS(fileUSS)
			if err != nil {
				return fmt.Errorf("ReadUSS: %w", err)
			}
		}

		if err := tk.LoadApp(appBinary, secret); err != nil {
			return fmt.Errorf("LoadApp failed: %w", err)
		}
	} else if enterUSS || fileUSS != "" {
		le.Printf("Warning: App already loaded. Use of USS not possible. Continuing with already loaded app...\n")
	}

	return nil
}

func genRandomData(randomGen RandomGen, genBytes int, filePath string) ([]byte, error) {
	var totRandom []byte
	var file *os.File
	var fileErr error
	var toFile bool

	if filePath != "" {
		toFile = true
		file, fileErr = os.Create(filePath)
		if fileErr != nil {
			return nil, fmt.Errorf("Could not create file %s: %w", filePath, fileErr)
		}
	}

	if !toFile {
		le.Printf("Random data follows on stdout...\n\n")
	} else {
		le.Printf("Writing random data to: %s\n", filePath)
	}

	left := genBytes
	for {
		get := left
		if get > RandomPayloadMaxBytes {
			get = RandomPayloadMaxBytes
		}
		random, err := randomGen.GetRandom(get)
		if err != nil {
			return nil, fmt.Errorf("GetRandom failed: %w", err)
		}
		totRandom = append(totRandom, random...)

		if toFile {
			_, err := file.Write(random)
			if err != nil {
				return nil, fmt.Errorf("Error could not write to file %w", err)
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

	file.Close()

	return totRandom, nil
}
