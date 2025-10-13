// SPDX-FileCopyrightText: 2025 Tillitis AB <tillitis.se>
// SPDX-License-Identifier: GPL-2.0-only

package reader

import (
	"fmt"
	"tkey-random-generator/generator"
)

type RandReader struct {
	generator generator.RandomGen
}

func New(r generator.RandomGen) RandReader {
	return RandReader{
		generator: r,
	}
}

func (r RandReader) Read(random []byte) (int, error) {
	if len(random) == 0 {
		return 0, nil
	}

	var n int
	var chunkSize int

	for toRead := len(random); toRead > 0; toRead -= chunkSize {
		chunkSize = min(generator.RandomPayloadMaxBytes, toRead)

		randomChunk, err := r.generator.GetRandom(chunkSize)
		if err != nil {
			return n, fmt.Errorf("GetRandom failed: %w", err)
		}

		copy(random[n:], randomChunk)
		n += chunkSize
	}

	return n, nil
}
