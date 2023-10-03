OBJCOPY ?= llvm-objcopy

LIBDIR ?= $(CURDIR)/../tkey-libs

CC = clang

INCLUDE=$(LIBDIR)/include

# If you want libcommon's qemu_puts() et cetera to output something on our QEMU
# debug port, remove -DNODEBUG below
CFLAGS = -target riscv32-unknown-none-elf -march=rv32iczmmul -mabi=ilp32 -mcmodel=medany \
   -static -std=gnu99 -O2 -ffast-math -fno-common -fno-builtin-printf \
   -fno-builtin-putchar -nostdlib -mno-relax -flto -g \
   -Wall -Werror=implicit-function-declaration \
   -I $(INCLUDE) -I $(LIBDIR)  \
   -DNODEBUG

AS = clang
ASFLAGS = -target riscv32-unknown-none-elf -march=rv32iczmmul -mabi=ilp32 -mcmodel=medany -mno-relax

LDFLAGS=-T $(LIBDIR)/app.lds -L $(LIBDIR) -lcommon -lcrt0

# Check for OS, if not macos assume linux
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
	shasum = shasum -a 512
	BUILD_CGO_ENABLED ?= 1
else
	shasum = sha512sum
	BUILD_CGO_ENABLED ?= 0
endif

.PHONY: all
all: random-generator/app.bin tkey-random-generator check-hash doc/tkey-random-generator.1

DESTDIR=/
PREFIX=/usr/local
destbin=$(DESTDIR)/$(PREFIX)/bin
destman1=$(DESTDIR)/$(PREFIX)/share/man/man1
.PHONY: install
install:
	install -Dm755 tkey-random-generator $(destbin)/tkey-random-generator
	strip $(destbin)/tkey-random-generator
	install -Dm644 doc/tkey-random-generator.1 $(destman1)/tkey-random-generator.1
	gzip -n9f $(destman1)/tkey-random-generator.1
.PHONY: uninstall
uninstall:
	rm -f \
	$(destbin)/tkey-random-generator \
	$(destman1)/tkey-random-generator.1.gz

podman:
	podman run --rm --mount type=bind,source=$(CURDIR),target=/src --mount type=bind,source=$(CURDIR)/../tkey-libs,target=/tkey-libs -w /src -it ghcr.io/tillitis/tkey-builder:2 make -j


# Turn elf into bin for device
%.bin: %.elf
	$(OBJCOPY) --input-target=elf32-littleriscv --output-target=binary $^ $@
	chmod a-x $@

check-hash: random-generator/app.bin
	cd random-generator && $(shasum) -c app.bin.sha512

# Random number generator app
RANDOMOBJS=random-generator/main.o random-generator/app_proto.o random-generator/rng.o random-generator/blake2s/blake2s.o
random-generator/app.elf: $(RANDOMOBJS)
	$(CC) $(CFLAGS) $(RANDOMOBJS) $(LDFLAGS) -L $(LIBDIR) -lmonocypher -o $@
$(RANDOMOBJS): $(INCLUDE)/tkey/tk1_mem.h random-generator/app_proto.h random-generator/rng.h random-generator/blake2s/blake2s.h

# Uses ../.clang-format
FMTFILES=random-generator/*.[ch]

.PHONY: fmt
fmt:
	clang-format --dry-run --ferror-limit=0 $(FMTFILES)
	clang-format --verbose -i $(FMTFILES)

.PHONY: checkfmt
checkfmt:
	clang-format --dry-run --ferror-limit=0 --Werror $(FMTFILES)

TKEY_RANDOM_GENERATOR_VERSION ?= $(shell git describe --dirty --always | sed -n "s/^v\(.*\)/\1/p")

# .PHONY to let go-build handle deps and rebuilds
.PHONY: tkey-random-generator
tkey-random-generator: random-generator/app.bin
	cp -af random-generator/app.bin cmd/tkey-random-generator/app.bin
	CGO_ENABLED=$(BUILD_CGO_ENABLED) go build -ldflags "-X main.version=$(TKEY_RANDOM_GENERATOR_VERSION)" -trimpath -o tkey-random-generator ./cmd/tkey-random-generator


doc/tkey-random-generator.1: doc/tkey-random-generator.scd
	scdoc < $^ > $@ 

.PHONY: clean
clean:
	rm -f random-generator/app.bin random-generator/app.elf $(RANDOMOBJS) \
	tkey-random-generator cmd/tkey-random-generator/app.bin gotools/golangci-lint


.PHONY: lint
lint:
	$(MAKE) -C gotools
	GOOS=linux   ./gotools/golangci-lint run
	GOOS=windows ./gotools/golangci-lint run

