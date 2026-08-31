.PHONY: build install test vet fmt check lint dist verify-dist release-check release-prepare pre-release live-check ci-smoke ci-cli-smoke ci-headless-smoke windows

VERSION ?= $(shell sed -n 's/^var Version = "\(.*\)"/\1/p' internal/config/config.go)

build:
	go build -ldflags "-X github.com/sspzoa/goppi/internal/config.Version=$(VERSION)" -o bin/goppi ./cmd/goppi

install:
	go install ./cmd/goppi

test:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

lint:
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/*.yml

check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	$(MAKE) lint
	go test -race ./...
	$(MAKE) build
	@test "$$(./bin/goppi version | awk '{print $$NF}')" = "$(VERSION)"
	$(MAKE) windows
	python3 -m py_compile scripts/ci-mock-solar.py scripts/ci-mock-mcp.py
	bash -n install.sh scripts/package.sh

windows:
	GOOS=windows GOARCH=amd64 go build ./...
	@for pkg in $$(go list ./...); do \
		GOOS=windows GOARCH=amd64 go test -c -o /dev/null $$pkg || exit 1; \
	done

dist:
	@mkdir -p dist
	@rm -f dist/goppi_*.tar.gz dist/SHA256SUMS dist/SHA256SUMS.sigstore.json
	bash scripts/package.sh "$(VERSION)" dist

verify-dist: dist
	@test "$$(grep -c '\.tar\.gz$$' dist/SHA256SUMS)" = "4"
	@test $$(ls -1 dist/goppi_*.tar.gz | wc -l | tr -d ' ') -eq 4
	@ver="$(VERSION)"; \
	for f in dist/goppi_*.tar.gz; do \
	  case "$$f" in *"_$${ver}_"*) ;; *) echo "stale tarball: $$f"; exit 1 ;; esac; \
	done
	@cd dist && (command -v sha256sum >/dev/null && sha256sum -c SHA256SUMS || shasum -a 256 -c SHA256SUMS)
	@ver="$(VERSION)"; \
	for os in darwin linux; do \
	  for arch in amd64 arm64; do \
	    name="goppi_$${ver}_$${os}_$${arch}"; \
	    contents=$$(tar -tzf "dist/$${name}.tar.gz"); \
	    test "$$contents" = "$${name}"; \
	  done; \
	done
	@ver="$(VERSION)"; \
	os=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	arch=$$(uname -m); \
	case "$$arch" in x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; esac; \
	if [ "$$os" = "linux" ]; then \
	  tmp=$$(mktemp -d); \
	  for a in amd64 arm64; do \
	    tar -xzf "dist/goppi_$${ver}_linux_$${a}.tar.gz" -C "$$tmp"; \
	    got=$$("$$tmp/goppi_$${ver}_linux_$${a}" version | awk '{print $$NF}'); \
	    test "$$ver" = "$$got"; \
	  done; \
	elif [ "$$os" = "darwin" ]; then \
	  tmp=$$(mktemp -d); \
	  tar -xzf "dist/goppi_$${ver}_darwin_$${arch}.tar.gz" -C "$$tmp"; \
	  got=$$("$$tmp/goppi_$${ver}_darwin_$${arch}" version | awk '{print $$NF}'); \
	  test "$$ver" = "$$got"; \
	fi
	@printf '%s\n' '{"mediaType":"fake"}' > dist/SHA256SUMS.sigstore.json
	GOPPI_DIST=$(CURDIR)/dist go test -race ./internal/release -run TestInstallScriptFromDistDir -count=1

release-check: check verify-dist

release-prepare: release-check ci-smoke

pre-release: release-prepare live-check

live-check:
	@if [ -z "$$UPSTAGE_API_KEY" ]; then \
	  echo "skip live-check: UPSTAGE_API_KEY unset"; \
	else \
	  GOPPI_LIVE=1 go test -race ./internal/provider -run Live -count=1; \
	fi

ci-smoke: ci-cli-smoke ci-headless-smoke

ci-cli-smoke: build
	@data=$${GOPPI_DATA_DIR:-$$(mktemp -d)}; \
	init=$${GOPPI_INIT_DIR:-$$data/goppi-init}; \
	export GOPPI_DATA_DIR="$$data" UPSTAGE_API_KEY=$${UPSTAGE_API_KEY:-ci-smoke}; \
	./bin/goppi help && \
	test "$$(./bin/goppi version | awk '{print $$NF}')" = "$(VERSION)" && \
	inspect=$$(./bin/goppi inspect --json); \
	echo "$$inspect" | grep -q '"has_key"' && \
	echo "$$inspect" | grep -q '"mode"' && \
	./bin/goppi inspect | grep -q key && \
	./bin/goppi complete commands | grep -qx init && \
	./bin/goppi complete efforts | grep -qx medium && \
	./bin/goppi complete formats | grep -qx json && \
	./bin/goppi complete slash /h | grep -qx /help && \
	./bin/goppi completions bash | grep -q 'login.*--stdin' && \
	./bin/goppi complete flags -p | grep -qx -- '-p' && \
	./bin/goppi complete flags --p | grep -qx -- '--prompt' && \
	./bin/goppi completions bash | grep -q goppi && \
	./bin/goppi models | grep -q solar-pro4 && \
	./bin/goppi doctor && \
	./bin/goppi sessions list && \
	./bin/goppi mcp list && \
	./bin/goppi worktree list && \
	./bin/goppi logout && \
	mkdir -p "$$init" && \
	./bin/goppi -C "$$init" init && \
	./bin/goppi -C "$$init" doctor

ci-headless-smoke: build
	@python3 -m py_compile scripts/ci-mock-solar.py scripts/ci-mock-mcp.py
	@port=$${MOCK_SOLAR_PORT:-19876}; \
	mock_pid=""; \
	trap 'kill $$mock_pid 2>/dev/null || true' EXIT; \
	MOCK_SOLAR_PORT=$$port python3 scripts/ci-mock-solar.py & mock_pid=$$!; \
	sleep 0.5; \
	data=$$(mktemp -d); \
	out=$$(GOPPI_DATA_DIR="$$data" GOPPI_BASE_URL="http://127.0.0.1:$$port" UPSTAGE_API_KEY=ci-smoke GOPPI_TUI=0 \
	  ./bin/goppi --output-format json -p smoke); \
	echo "$$out" | grep -q '"text".*ok' && \
	echo "$$out" | grep -qE '"session_id"[[:space:]]*:[[:space:]]*"[0-9a-f]{16}"' && \
	echo "$$out" | grep -q '"usage"' && \
	echo "$$out" | grep -q '"mode"' && \
	echo "$$out" | grep -q '"workdir"' && \
	echo "$$out" | grep -q '"worktree"' && \
	echo "$$out" | grep -q '"reasoning"' && \
	sid=$$(printf '%s' "$$out" | sed -n 's/.*"session_id"[[:space:]]*:[[:space:]]*"\([0-9a-f]\{16\}\)".*/\1/p' | head -1) && \
	test -n "$$sid" && \
	GOPPI_DATA_DIR="$$data" ./bin/goppi export "$$sid" | grep -q smoke
