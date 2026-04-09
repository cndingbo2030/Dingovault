# Dingovault production builds (requires Wails CLI: https://wails.io)
.PHONY: build release dev clean benchmark fmt lint-frontend dist dist-dmg deploy-saas

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
APP_VERSION ?= 1.1.0-beta
GO_LDFLAGS_X := -X github.com/cndingbo2030/dingovault/internal/version.String=$(APP_VERSION)
DIST_ARCH ?= $(shell uname -m)
DIST_BUNDLE = dingovault-$(VERSION)-darwin-$(DIST_ARCH)

build:
	wails build -clean -ldflags="-s -w $(GO_LDFLAGS_X)"

release:
	wails build -clean -ldflags="-s -w $(GO_LDFLAGS_X)"

dev:
	wails dev

clean:
	rm -rf build/bin

benchmark:
	go run ./scripts/benchmark.go

# Stress + integrity check with encrypted SQLite (DINGO_MASTER_KEY must be ≥16 chars).
benchmark-encrypted:
	DINGO_MASTER_KEY=dingovault-bench-encryption-key-min16 go run ./scripts/benchmark.go -files 12 -total 2400 -verify

fmt:
	go fmt ./...

lint-frontend:
	cd frontend && npm run lint

# Pack release app bundle + default Dingovault-Help vault into dist/$(DIST_BUNDLE).zip
dist: release
	rm -rf dist/$(DIST_BUNDLE)
	mkdir -p dist/$(DIST_BUNDLE)
	cp -R build/bin/dingovault.app dist/$(DIST_BUNDLE)/
	cp -R vaults/Dingovault-Help dist/$(DIST_BUNDLE)/Dingovault-Help
	cp -R demo-vault dist/$(DIST_BUNDLE)/demo-vault
	cd dist && rm -f $(DIST_BUNDLE).zip && zip -rq $(DIST_BUNDLE).zip $(DIST_BUNDLE)
	@echo "Created dist/$(DIST_BUNDLE).zip"

# macOS disk image (requires dist folder from dist target)
dist-dmg: dist
	hdiutil create -volname "Dingovault" -srcfolder dist/$(DIST_BUNDLE) -ov -format UDZO dist/$(DIST_BUNDLE).dmg
	@echo "Created dist/$(DIST_BUNDLE).dmg"

# Lightweight Linux SaaS API image (Alpine, non-root, SQLite volume at /data)
deploy-saas:
	docker build -t dingovault-saas:latest -f Dockerfile .
	@echo "Built dingovault-saas:latest — run: docker run --rm -p 12030:12030 -v dingovault-data:/data dingovault-saas:latest"
