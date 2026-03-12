// Copyright (C) 2026 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

//go:build !dev

package main

import _ "embed"

// nolint:typecheck // Avoid lint error when the embedding file is missing.
//
//go:embed random-generator.bin-v0.0.2
var appBinary []byte

const appName string = "random-generator v0.0.2"
