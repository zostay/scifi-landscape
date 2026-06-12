# ebiten's macOS Metal driver calls deprecated CoreVideo CVDisplayLink APIs, so
# cgo prints a wall of deprecation warnings on every build. Silence them here so
# the console stays clean. (They are harmless — ebiten still works fine.)
export CGO_CFLAGS = -Wno-deprecated-declarations

# Use bash for stderr filtering below (process substitution). The run target
# drops these benign macOS Metal/CoreAnimation log lines that ebiten can't
# suppress — they are printed straight to stderr at runtime (e.g. while the
# window is resized or occluded). Add -e patterns if new noise appears.
SHELL       := /bin/bash
METAL_NOISE := -e 'CAMetalLayer nextDrawable'

GO   ?= go
ARGS ?=

.PHONY: build run render test vet fmt clean

# Build the windowed app binary.
build:
	$(GO) build -o scifi-landscape .

# Run the windowed app, filtering benign Metal noise from stderr while keeping
# real errors and the exit code. Pass flags via ARGS, e.g.
#   make run ARGS="-s 7 -t dusk"
#   make run ARGS="config scene.png"
run:
	$(GO) run . $(ARGS) 2> >(grep -F --line-buffered -v $(METAL_NOISE) >&2); ec=$$?; wait; exit $$ec

# Render a scene headlessly, e.g. make render ARGS="-s 7 -o scene.png".
render:
	$(GO) run ./cmd/render $(ARGS)

# Run the test suite.
test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

# Remove build artifacts.
clean:
	$(GO) clean
	rm -f scifi-landscape render
