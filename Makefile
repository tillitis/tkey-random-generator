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

LDFLAGS=-T $(LIBDIR)/app.lds -L $(LIBDIR)/libcommon/ -lcommon -L $(LIBDIR)/libcrt0/ -lcrt0

.PHONY: all
all: random-generator/app.bin runrandom

podman:
	podman run --rm --mount type=bind,source=$(CURDIR),target=/src --mount type=bind,source=$(CURDIR)/../tkey-libs,target=/tkey-libs -w /src -it ghcr.io/tillitis/tkey-builder:2 make -j


# Turn elf into bin for device
%.bin: %.elf
	$(OBJCOPY) --input-target=elf32-littleriscv --output-target=binary $^ $@
	chmod a-x $@

# Random number generator app
RANDOMOBJS=random-generator/main.o random-generator/app_proto.o random-generator/rng.o random-generator/blake2s/blake2s.o
random-generator/app.elf: $(RANDOMOBJS)
	$(CC) $(CFLAGS) $(RANDOMOBJS) $(LDFLAGS) -L $(LIBDIR)/monocypher -lmonocypher -o $@
$(RANDOMOBJS): $(INCLUDE)/tk1_mem.h random-generator/app_proto.h random-generator/rng.h random-generator/blake2s/blake2s.h

# Uses ../.clang-format
FMTFILES=random-generator/*.[ch]

.PHONY: fmt
fmt:
	clang-format --dry-run --ferror-limit=0 $(FMTFILES)
	clang-format --verbose -i $(FMTFILES)

.PHONY: checkfmt
checkfmt:
	clang-format --dry-run --ferror-limit=0 --Werror $(FMTFILES)

# .PHONY to let go-build handle deps and rebuilds
.PHONY: runrandom
runrandom: random-generator/app.bin
	cp -af random-generator/app.bin client-app/app.bin
	go build -o runrandom ./client-app


.PHONY: clean
clean:
	rm -f random-generator/app.bin random-generator/app.elf $(RANDOMOBJS) \
	runrandom client-app/app.bin gotools/golangci-lint


.PHONY: lint
lint:
	$(MAKE) -C gotools
	GOOS=linux   ./gotools/golangci-lint run
	GOOS=windows ./gotools/golangci-lint run

