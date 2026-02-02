# ant.build Frontend Makefile
# Build and deploy Hugo site to Cloudflare Pages

.PHONY: help install clean build build-staging build-production dev serve test lint

# Configuration
HUGO := hugo
PUBLIC_DIR := public
STAGING_URL := https://staging.ant.build
PRODUCTION_URL := https://ant.build
STAGING_GATEWAY := https://api-staging.ant.build
PRODUCTION_GATEWAY := https://api.ant.build

# Default target
help:
	@echo "ant.build Frontend Build Commands"
	@echo ""
	@echo "Development:"
	@echo "  make dev              - Start development server with drafts"
	@echo "  make serve            - Start production preview server"
	@echo "  make clean            - Remove build artifacts"
	@echo ""
	@echo "Build:"
	@echo "  make build            - Build for local/default environment"
	@echo "  make build-staging    - Build for Cloudflare Pages staging"
	@echo "  make build-production - Build for Cloudflare Pages production"
	@echo ""
	@echo "Quality:"
	@echo "  make lint             - Check for Hugo warnings"
	@echo "  make test             - Validate build output"
	@echo ""
	@echo "Cloudflare Pages:"
	@echo "  make cf-deploy-staging    - Deploy to staging (requires wrangler)"
	@echo "  make cf-deploy-production - Deploy to production (requires wrangler)"

# Install Hugo (for CI environments)
install:
	@echo "Checking Hugo installation..."
	@which hugo > /dev/null || (echo "Hugo not found. Install from https://gohugo.io/installation/" && exit 1)
	@hugo version

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(PUBLIC_DIR)
	rm -rf resources/_gen
	rm -f .hugo_build.lock

# Development server with live reload and drafts
dev:
	@echo "Starting development server..."
	$(HUGO) server -D --bind 0.0.0.0 --port 1313 --navigateToChanged

# Production preview server
serve:
	@echo "Starting production preview server..."
	$(HUGO) server --bind 0.0.0.0 --port 1313 --minify

# Default build (local environment)
build: clean
	@echo "Building site for local environment..."
	$(HUGO) --gc --minify
	@echo "Build complete: $(PUBLIC_DIR)/"

# Staging build for Cloudflare Pages
build-staging: clean
	@echo "Building site for STAGING environment..."
	@echo "Base URL: $(STAGING_URL)"
	@echo "Gateway:  $(STAGING_GATEWAY)"
	HUGO_ENV=staging \
	HUGO_PARAMS_GATEWAYURL=$(STAGING_GATEWAY) \
	$(HUGO) \
		--gc \
		--minify \
		--baseURL $(STAGING_URL) \
		--environment staging
	@echo ""
	@echo "Staging build complete: $(PUBLIC_DIR)/"
	@echo "Pages: $$(find $(PUBLIC_DIR) -name '*.html' | wc -l)"

# Production build for Cloudflare Pages
build-production: clean
	@echo "Building site for PRODUCTION environment..."
	@echo "Base URL: $(PRODUCTION_URL)"
	@echo "Gateway:  $(PRODUCTION_GATEWAY)"
	HUGO_ENV=production \
	HUGO_PARAMS_GATEWAYURL=$(PRODUCTION_GATEWAY) \
	$(HUGO) \
		--gc \
		--minify \
		--baseURL $(PRODUCTION_URL) \
		--environment production
	@echo ""
	@echo "Production build complete: $(PUBLIC_DIR)/"
	@echo "Pages: $$(find $(PUBLIC_DIR) -name '*.html' | wc -l)"

# Lint - check for Hugo warnings
lint:
	@echo "Checking for Hugo warnings..."
	$(HUGO) --gc 2>&1 | grep -i "warn" && exit 1 || echo "No warnings found"

# Test - validate build output
test: build
	@echo "Validating build output..."
	@test -f $(PUBLIC_DIR)/index.html || (echo "ERROR: index.html not found" && exit 1)
	@test -f $(PUBLIC_DIR)/sitemap.xml || (echo "ERROR: sitemap.xml not found" && exit 1)
	@test -d $(PUBLIC_DIR)/features || (echo "ERROR: features directory not found" && exit 1)
	@test -d $(PUBLIC_DIR)/pricing || (echo "ERROR: pricing directory not found" && exit 1)
	@test -d $(PUBLIC_DIR)/jobs || (echo "ERROR: jobs directory not found" && exit 1)
	@test -d $(PUBLIC_DIR)/blog || (echo "ERROR: blog directory not found" && exit 1)
	@test -d $(PUBLIC_DIR)/provision || (echo "ERROR: provision directory not found" && exit 1)
	@echo "All validations passed!"

# Cloudflare Pages deployment - Staging
# Requires: npm install -g wrangler && wrangler login
cf-deploy-staging: build-staging
	@echo "Deploying to Cloudflare Pages (staging)..."
	@which wrangler > /dev/null || (echo "wrangler not found. Run: npm install -g wrangler" && exit 1)
	wrangler pages deploy $(PUBLIC_DIR) --project-name=ant-build-staging

# Cloudflare Pages deployment - Production
# Requires: npm install -g wrangler && wrangler login
cf-deploy-production: build-production
	@echo "Deploying to Cloudflare Pages (production)..."
	@which wrangler > /dev/null || (echo "wrangler not found. Run: npm install -g wrangler" && exit 1)
	wrangler pages deploy $(PUBLIC_DIR) --project-name=ant-build --branch=main

# Build info
info:
	@echo "Hugo version:"
	@$(HUGO) version
	@echo ""
	@echo "Environment variables:"
	@echo "  HUGO_ENV=$(HUGO_ENV)"
	@echo "  HUGO_PARAMS_GATEWAYURL=$(HUGO_PARAMS_GATEWAYURL)"
	@echo ""
	@echo "Configuration:"
	@echo "  Staging URL:    $(STAGING_URL)"
	@echo "  Production URL: $(PRODUCTION_URL)"
	@echo "  Staging API:    $(STAGING_GATEWAY)"
	@echo "  Production API: $(PRODUCTION_GATEWAY)"
