// Copyright (C) 2022 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

#ifndef RNG_H
#define RNG_H

#include <stdint.h>
#include <stddef.h>

// state context
void rng_init();
int rng_get(uint8_t *dst, size_t sz);

#endif
