// SPDX-FileCopyrightText: 2025 Tillitis AB <tillitis.se>
// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"flag"
	"fmt"
	"os"
	"tkey-random-generator/generator"
	"tkey-random-generator/reader"

	"github.com/sethvargo/go-diceware/diceware"
	"github.com/tillitis/tkeyclient"
)

func main() {
	wordsN := flag.Int("words", 7, "Number of words to generate")

	flag.Parse()

	tkeyclient.SilenceLogging()

	devPath, err := tkeyclient.DetectSerialPort(true)
	if err != nil {
		fmt.Printf("Couldn't find a Tkey", err)
		os.Exit(1)
	}

	tk := tkeyclient.New()

	fmt.Printf("Connecting to device on serial port %s...\n", devPath)
	if err := tk.Connect(devPath, tkeyclient.WithSpeed(tkeyclient.SerialSpeed)); err != nil {
		fmt.Printf("could not open %s: %w", devPath, err)
		os.Exit(1)
	}

	gen := generator.New(tk)

	r := reader.New(gen)

	genInput := diceware.GeneratorInput{
		WordList:   diceware.WordListEffLarge(),
		RandReader: r,
	}

	diceGen, err := diceware.NewGenerator(&genInput)
	if err != nil {
		panic(err)
	}

	words, err := diceGen.Generate(*wordsN)
	if err != nil {
		panic(err)
	}

	for _, word := range words {
		fmt.Printf("%v ", word)
	}

	fmt.Println("")
}
