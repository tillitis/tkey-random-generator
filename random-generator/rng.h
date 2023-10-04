// Copyright (C) 2022 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

#ifndef RNG_H
#define RNG_H

#include <stdint.h>

// state context
typedef struct {
	uint32_t state_ctr_lsb;
	uint32_t state_ctr_msb;
	uint32_t reseed_ctr;
	uint32_t state[16];
	uint32_t digest[8];
} rng_ctx;

void rng_init(rng_ctx *ctx);
int rng_get(uint32_t *output, rng_ctx *ctx, int bytes);

#endif
