# Dingovault production builds (requires Wails CLI: https://wails.io)
# Semantic release filenames align with .github/workflows/release.yml (substitute REF when tagging).
.PHONY: build release release-server dev clean benchmark fmt lint-frontend dist dist-dmg deploy-saas

REF ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
APP_VERSION ?= 1.4.1
GO_LDFLAGS_X := -X github.com/cndingbo2030/dingovault/internal/version.String=$(APP_VERSION)
DIST_ARCH ?= $(shell uname -m)

# Local / CI semantic artifact names (REF should be the tag, e.g. v1.4.1)
RELEASE_LINUX_DESKTOP := Dingovault-$(REF)-Linux-Desktop-amd64.tar.gz
RELEASE_LINUX_SERVER := Dingovault-$(REF)-Linux-Server-amd64
RELEASE_WIN_SERVER := Dingovault-$(REF)-Windows-Server-amd64.exe
RELEASE_WIN_INSTALLER := Dingovault-$(REF)-Windows-64bit-Installer.exe
RELEASE_MAC_INTEL := Dingovault-$(REF)-macOS-Intel-Processor.zip
RELEASE_MAC_AS := Dingovault-$(REF)-macOS-Apple-Silicon-M1-M2-M3.zip
RELEASE_MAC_SERVER_INTEL := Dingovault-$(REF)-macOS-Server-Intel-amd64
RELEASE_MAC_SERVER_AS := Dingovault-$(REF)-macOS-Server-Apple-Silicon-arm64
RELEASE_ANDROID_AAR := Dingovault-$(REF)-Android-Library-AAR.aar
RELEASE_ANDROID_APK := Dingovault-$(REF)-Android-Mobile-Phone-Tablet.apk
RELEASE_ANDROID_AAB := Dingovault-$(REF)-Android-Play-Bundle.aab

DIST_BUNDLE = dingovault-$(VERSION)-darwin-$(DIST_ARCH)
# Local ad-hoc server binary (CI uses Dingovault-$(REF)-*-Server-* names from release.yml).
SERVER_BIN = dingovault-v$(APP_VERSION)-$(shell uname -s | tr '[:upper:]' '[:lower:]')-$(DIST_ARCH)

build:
	wails build -clean -ldflags="-s -w $(GO_LDFLAGS_X)"
	chmod +x build/bin/dingovault.app/Contents/MacOS/Dingovault || true

release:
	wails build -clean -ldflags="-s -w $(GO_LDFLAGS_X)"
	chmod +x build/bin/dingovault.app/Contents/MacOS/Dingovault || true

release-server:
	go build -trimpath -ldflags="-s -w $(GO_LDFLAGS_X)" -o $(SERVER_BIN) ./cmd/dingovault

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

# Echo canonical release filenames (handy when cutting a release locally).
.PHONY: release-names
release-names:
	@echo "$(RELEASE_LINUX_DESKTOP)"
	@echo "$(RELEASE_LINUX_SERVER)"
	@echo "$(RELEASE_WIN_SERVER)"
	@echo "$(RELEASE_WIN_INSTALLER)"
	@echo "$(RELEASE_MAC_INTEL)"
	@echo "$(RELEASE_MAC_AS)"
	@echo "$(RELEASE_MAC_SERVER_INTEL)"
	@echo "$(RELEASE_MAC_SERVER_AS)"
	@echo "$(RELEASE_ANDROID_AAR)"
	@echo "$(RELEASE_ANDROID_APK)"
	@echo "$(RELEASE_ANDROID_AAB)"
