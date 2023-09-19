// Copyright (C) 2022, 2023 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"bytes"
	"crypto/ed25519"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
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
	var fileUSS, devPath, filePath, fileRandData, fileSignature, filePubkey string
	var speed, genBytes int
	var enterUSS, helpOnlyGen, helpOnlyVerify, shouldSign, isBinary bool

	genString := "generate"
	verifyString := "verify"

	// Default flags to show
	root := pflag.NewFlagSet("root", pflag.ExitOnError)
	root.Usage = func() {
		desc := fmt.Sprintf(`%[1]s fetches random numbers from the TRNG on the
Tillitis' TKey. This program embeds the random generator-app binary,
which it loads onto the TKey and starts.

The generated data can be signed with an Ed25519 private key. See more
information under the subcommand "generate".

It is also possible to verify previously generated random data,
by providing the signature, random data, and public key. Verification
can be performed without a TKey connected. See more information under
the subcommand "verify".

Usage:
  %[1]s <command> [flags] FILE...

Commands:
  generate    Generate random data
  verify      Verify signature of previously generated data

Use <command> --help for further help, i.e. %[1]s verify --help`, os.Args[0])
		le.Printf("%s\n\n%s", desc,
			root.FlagUsagesWrapped(86))
	}

	// Flag for command "generate"
	cmdGen := pflag.NewFlagSet(genString, pflag.ExitOnError)
	cmdGen.SortFlags = false
	cmdGen.StringVarP(&devPath, "port", "p", "",
		"Set serial port device `PATH`. If this is not passed, auto-detection will be attempted.")
	cmdGen.IntVar(&speed, "speed", tkeyclient.SerialSpeed,
		"Set serial port speed in `BPS` (bits per second).")
	cmdGen.BoolVarP(&shouldSign, "signature", "s", false, "Get the signature of the generated random data.")
	cmdGen.StringVarP(&filePath, "file", "f", "",
		"Output random data as binary to `FILE`.")
	cmdGen.BoolVarP(&helpOnlyGen, "help", "h", false, "Output this help.")
	cmdGen.BoolVar(&enterUSS, "uss", false,
		"Enable typing of a phrase to be hashed as the User Supplied Secret. The USS is loaded onto the TKey along with the app itself. A different USS results in different Compound Device Identifier, different start of the random sequence, and another key pair used for signing.")
	cmdGen.StringVar(&fileUSS, "uss-file", "",
		"Read `FILE` and hash its contents as the USS. Use '-' (dash) to read from stdin. The full contents are hashed unmodified (e.g. newlines are not stripped).")

	cmdGen.Usage = func() {
		desc := fmt.Sprintf(`Usage %[1]s generate <bytes> [-s] [--uss] [flags..]

  Generates amount of data specified with <bytes> and optionally create a signature
  to make it possible to provide proof of the origin. The generated random data is
  first hashed using BLAKE2s, and then signed with and Ed25519 private key.

  Output can be chosen between stdout (hex) and a binary file.

  Usage:`, os.Args[0])
		le.Printf("%s\n\n%s", desc,
			cmdGen.FlagUsagesWrapped(80))
	}

	// Flag for command "verify"
	cmdVerify := pflag.NewFlagSet(verifyString, pflag.ExitOnError)
	cmdVerify.SortFlags = false
	cmdVerify.BoolVarP(&isBinary, "binary", "b", false, "Specify if the input FILE is in binary format.")
	cmdVerify.BoolVarP(&helpOnlyVerify, "help", "h", false, "Output this help.")
	cmdVerify.Usage = func() {
		desc := fmt.Sprintf(`Usage: %[1]s verify FILE SIG-FILE PUBKEY-FILE [-b]

  Verifies whether the Ed25519 signature of the message is valid.
  Does not need a connected TKey to verify.

  First the message, FILE, is hashed using BLAKE2s, then the signature
  is verified with the message and the public key.

  FILE is either a binary or a hex representation of the random data.
  SIG-FILE is expected to be an 64 bytes Ed25519 signature in hex.
  PUBKEY-FILE is expected to be an 32 bytes Ed25519 public key in hex.

  The return value is 0 if the signature is valid, otherwise non-zero.
  Newlines will be striped from the input files. `, os.Args[0])
		le.Printf("%s\n\n%s", desc,
			cmdVerify.FlagUsagesWrapped(86))
	}

	// No arguments, print and exit
	if len(os.Args) == 1 {
		root.Usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case genString:
		if err := cmdGen.Parse(os.Args[2:]); err != nil {
			le.Printf("Error parsing input arguments: %v\n", err)
			os.Exit(2)
		}

		if helpOnlyGen {
			cmdGen.Usage()
			os.Exit(0)
		}

		if enterUSS && fileUSS != "" {
			le.Printf("Pass only one of --uss or --uss-file.\n\n")
			pflag.Usage()
			os.Exit(2)
		}

		if cmdGen.NArg() < 1 {
			le.Printf("Bytes to generate required.\n\n")
			cmdGen.Usage()
			os.Exit(2)
		} else if cmdGen.NArg() > 1 {
			le.Printf("Unexpected argument: %s\n\n", strings.Join(os.Args[3:], " "))
			cmdGen.Usage()
			os.Exit(2)
		}

		var err error
		genBytes, err = strconv.Atoi(cmdGen.Args()[0])
		if err != nil {
			le.Printf("Argument needs to be integer.\n\n")
			cmdGen.Usage()
			os.Exit(2)
		}

		err = generate(devPath, enterUSS, fileUSS, speed, genBytes, filePath, shouldSign)
		if err != nil {
			le.Printf("Error generating random data: %v\n", err)
			os.Exit(1)
		}

		os.Exit(0)
	case verifyString:
		if err := cmdVerify.Parse(os.Args[2:]); err != nil {
			le.Printf("Error parsing input arguments: %v\n", err)
			os.Exit(2)
		}

		if helpOnlyVerify {
			cmdVerify.Usage()
			os.Exit(0)
		}

		if cmdVerify.NArg() < 3 {
			le.Printf("Missing %d input file(s) to verify signature.\n\n", 3-cmdVerify.NArg())
			cmdVerify.Usage()
			os.Exit(2)
		} else if cmdVerify.NArg() > 3 {
			le.Printf("Unexpected argument: %s\n\n", strings.Join(cmdVerify.Args()[3:], " "))
			cmdVerify.Usage()
			os.Exit(2)
		}
		fileRandData = cmdVerify.Args()[0]
		fileSignature = cmdVerify.Args()[1]
		filePubkey = cmdVerify.Args()[2]

		le.Printf("Verifying signature ...\n")
		if err := verifySignature(fileRandData, fileSignature, filePubkey, isBinary); err != nil {
			le.Printf("Error verifying: %v\n", err)
			os.Exit(1)
		}
		le.Printf("Signature verified.\n")

		os.Exit(0)
	default:
		root.Usage()
		le.Printf("%q is not a valid subcommand.\n", os.Args[1])
		os.Exit(2)
	}
	os.Exit(1) // should never be reached
}

// subcommand to generate random data
func generate(devPath string, enterUSS bool, fileUSS string, speed int, genBytes int, filePath string, shouldSign bool) error {
	tkeyclient.SilenceLogging()

	if devPath == "" {
		var err error
		devPath, err = tkeyclient.DetectSerialPort(true)
		if err != nil {
			return fmt.Errorf("DetectSerialPort: %w", err)
		}
	}

	tk := tkeyclient.New()
	le.Printf("Connecting to device on serial port %s...\n", devPath)
	if err := tk.Connect(devPath, tkeyclient.WithSpeed(speed)); err != nil {
		return fmt.Errorf("could not open %s: %w", devPath, err)
	}

	randomGen := New(tk)
	exit := func(code int) {
		if err := randomGen.Close(); err != nil {
			le.Printf("%v\n", err)
		}
		os.Exit(code)
	}
	handleSignals(func() { exit(1) }, os.Interrupt, syscall.SIGTERM)
	defer randomGen.Close()

	err := loadApp(tk, enterUSS, fileUSS)
	if err != nil {
		return fmt.Errorf("couldn't load app: %w", err)
	}

	if !isWantedApp(randomGen) {
		return fmt.Errorf("the TKey may already be running an app, but not the expected. Please unplug and plug it in again")
	}

	totRandom, err := genRandomData(randomGen, genBytes, filePath)
	if err != nil {
		return fmt.Errorf("genRandomData failed: %w", err)
	}

	// Always fetch the signature and hash to re-init the hash on the TKey
	signature, hash, err := randomGen.GetSignature()
	if err != nil {
		return fmt.Errorf("GetSig failed: %w", err)
	}

	// Only print and verify if asked
	if shouldSign {
		pubkey, err := randomGen.GetPubkey()
		if err != nil {
			return fmt.Errorf("GetPubkey failed: %w", err)
		}

		fmt.Printf("Public key: %x\n", pubkey)
		fmt.Printf("Signature: %x\n", signature)
		fmt.Printf("Hash: %x\n", hash)

		// Do we compute the same hash digest as random-generator did?
		errHash := verifyHash(hash, totRandom)
		if errHash != nil {
			return fmt.Errorf("hash FAILED verification: %w", errHash)
		}

		le.Print(("\nVerifying signature ... "))
		if !ed25519.Verify(pubkey, hash, signature) {
			return fmt.Errorf("signature FAILED verification")
		}
		le.Printf("signature verified.\n")
	}

	return nil
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

// genRandomData fetches genBytes bytes of random data and either prints to a file or stdout
func genRandomData(randomGen RandomGen, genBytes int, filePath string) ([]byte, error) {
	var totRandom []byte
	var file *os.File
	var fileErr error
	var toFile bool

	if filePath != "" {
		toFile = true
		file, fileErr = os.Create(filePath)
		if fileErr != nil {
			return nil, fmt.Errorf("could not create file %s: %w", filePath, fileErr)
		}
	}

	if !toFile {
		le.Printf("Random data follows on stdout...\n\n")
	} else {
		le.Printf("Writing %d bytes of random data to: %s\n", genBytes, filePath)
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
				return nil, fmt.Errorf("error could not write to file %w", err)
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

// fileInputToHex reads inputFile and returns a trimmed slice decoded to hex.
func fileInputToHex(inputFile string) ([]byte, error) {
	input, err := os.ReadFile(inputFile)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", inputFile, err)
	}

	input = bytes.Trim(input, "\n")
	hexOutput := make([]byte, hex.DecodedLen(len(input)))
	_, err = hex.Decode(hexOutput, input)
	if err != nil {
		return nil, fmt.Errorf("could not decode: %w", err)
	}
	return hexOutput, nil
}

// verifySignature verifies a Ed25519 signature from input files of message, signature and public key
func verifySignature(fileRandData string, fileSignature string, filePubkey string, isBinary bool) error {
	signature, err := fileInputToHex(fileSignature)
	if err != nil {
		return fmt.Errorf("fileInputToHex failed: %w", err)
	}

	if len(signature) != 64 {
		return fmt.Errorf("invalid length of signature. Expected 64 bytes, got %d bytes", len(signature))
	}

	pubkey, err := fileInputToHex(filePubkey)
	if err != nil {
		return fmt.Errorf("fileInputToHex failed: %w", err)
	}

	if len(pubkey) != 32 {
		return fmt.Errorf("invalid length of public key. Expected 32 bytes, got %d bytes", len(pubkey))
	}

	fmt.Printf("Public key: %x\n", pubkey)
	fmt.Printf("Signature: %x\n", signature)

	var message []byte
	if isBinary {

		message, err = os.ReadFile(fileRandData)
		if err != nil {
			return fmt.Errorf("could not read %s: %w", fileRandData, err)
		}
	} else {
		message, err = fileInputToHex(fileRandData)
		if err != nil {
			return fmt.Errorf("fileInputToHex failed: %w", err)
		}
	}

	digest := doHash(message)
	le.Printf("BLAKE2s hash: %x\n", digest)

	if !ed25519.Verify(pubkey, digest[:], signature) {
		return fmt.Errorf("signature not valid")
	}

	return nil
}

// verifyHash returns error if the hash and raw data hashed is not equal
func verifyHash(hash []byte, randomData []byte) error {
	localHash := doHash(randomData)

	if !bytes.Equal(hash, localHash[:]) {
		return fmt.Errorf("hash not equal")
	}
	return nil
}

// doHash returns a blake2s hash of input
func doHash(data []byte) [32]byte {
	return blake2s.Sum256(data)
}
