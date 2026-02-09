export GIT_REVISION=$(shell git rev-parse --short HEAD)

# Packages to exclude from testing (examples, benchmarks)
TEST_EXCLUDE := examples tests/benchmarks

.PHONY: test
test:
	gotestsum --junitfile /tmp/test-reports/unit-tests.xml \
		-- -coverprofile=coverage.out -covermode=atomic \
		$$(go list ./... | grep -v -E '$(subst $(eval ) ,|,$(TEST_EXCLUDE))')

.PHONY: pprof
pprof:
	go build
	./risor --cpu-profile cpu.out ./examples/scripts/fibonacci.risor
	go tool pprof -http=:8080 ./cpu.out

.PHONY: bench
bench:
	go test -bench=. -benchmem ./tests/benchmarks/go

# https://code.visualstudio.com/api/working-with-extensions/publishing-extension#packaging-extensions
.PHONY: install-tools
install-tools:
	npm install -g vsce typescript

.PHONY: extension-login
extension-login:
	npx vsce login $(VSCE_LOGIN)

.PHONY: extension-package
extension-package:
	cd vscode && npx vsce package

.PHONY: extension-install
extension-install:
	go install ./cmd/risor-lsp
	$(MAKE) extension-package
	cd vscode && code --install-extension risor-*.vsix

.PHONY: extension-publish
extension-publish:
	$(MAKE) extension-package
	cd vscode && npx vsce publish

.PHONY: tidy
tidy:
	find . -name go.mod -execdir go mod tidy \;
	go work sync

.PHONY: update-deps
update-deps:
	find . -name go.mod -execdir go get -u ./... \;
	find . -name go.mod -execdir go mod tidy \;

.PHONY: cover
cover:
	go test -coverprofile cover.out ./...
	go tool cover -html=cover.out

.PHONY: format
format:
	gofumpt -l -w .

.PHONY: release
release:
	goreleaser release --clean -p 2

.PHONY: generate
generate:
	go generate
	gofumpt -l -w .

# Use entr to watch for changes to markdown files and copy them to the
# risor-site repo (expected to be at ../risor-site). You can brew install entr.
# Then in the risor-site repo in a separate terminal, run `npm run dev` and
# open http://localhost:3000/docs in your browser for hot reloaded docs updates.
.PHONY: docs-dev
docs-dev:
	find . -name "*.md" | entr go run ./cmd/risor-docs

.PHONY: docker-build-init
docker-build-init:
	docker buildx create --use --name builder --platform linux/arm64,linux/amd64
	docker buildx inspect --bootstrap

.PHONY: docker-build
docker-build:
	docker buildx build \
		-t risor/risor:latest \
		-t risor/risor:$(GIT_REVISION) \
		-t risor/risor:2.0.0 \
		--build-arg "RISOR_VERSION=2.0.0" \
		--build-arg "GIT_REVISION=$(GIT_REVISION)" \
		--build-arg "BUILD_DATE=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')" \
		--platform linux/amd64,linux/arm64 \
		--push .
