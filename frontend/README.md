# ant.build Frontend

A Hugo-based marketing and documentation site for the ant.build AI code generation platform.

## Features

- **Homepage** with prompt builder for code generation
- **Features page** showcasing platform capabilities
- **Pricing page** with transparent pricing (code gen is free!)
- **Jobs page** with working search and filter functionality
- **Blog** with sample posts about AI development
- **Provision page** for hardware selection and deployment

## Quick Start

```bash
# Show available commands
make help

# Development server with live reload
make dev

# Build for production
make build-production

# Build for staging
make build-staging
```

## Project Structure

```
frontend/
├── config/            # Environment-specific configs
│   ├── staging/      # Staging overrides
│   └── production/   # Production overrides
├── content/           # Markdown content
│   ├── blog/         # Blog posts
│   ├── docs/         # Documentation
│   ├── features/     # Features page
│   ├── jobs/         # Careers page
│   ├── pricing/      # Pricing page
│   └── provision/    # Deployment page
├── static/           # Static assets
│   ├── css/         # Additional CSS
│   ├── js/          # JavaScript files
│   └── images/      # Images and favicon
├── themes/antbuild/  # Custom theme
│   ├── layouts/     # Hugo templates
│   ├── static/      # Theme assets
│   └── theme.toml   # Theme metadata
├── hugo.toml        # Hugo configuration
├── Makefile         # Build commands
└── wrangler.toml    # Cloudflare Pages config
```

## Configuration

Key configuration in `hugo.toml`:

- `params.gatewayURL`: API endpoint for the ant.build gateway
- `params.enablePromptBuilder`: Toggle prompt builder feature
- `params.enableJobs`: Toggle jobs page
- `params.enableBlog`: Toggle blog section

Environment-specific overrides are in `config/<environment>/hugo.toml`.

## Makefile Commands

```bash
# Development
make dev              # Start dev server with drafts (port 1313)
make serve            # Start production preview server
make clean            # Remove build artifacts

# Building
make build            # Build for local/default environment
make build-staging    # Build for Cloudflare Pages staging
make build-production # Build for Cloudflare Pages production

# Quality
make lint             # Check for Hugo warnings
make test             # Validate build output

# Cloudflare Pages
make cf-deploy-staging    # Deploy to staging
make cf-deploy-production # Deploy to production
```

## Deployment to Cloudflare Pages

### Prerequisites

1. Install Hugo (v0.123+):
   ```bash
   # macOS
   brew install hugo

   # Ubuntu/Debian
   sudo apt install hugo

   # Or download from https://gohugo.io/installation/
   ```

2. Install Wrangler CLI:
   ```bash
   npm install -g wrangler
   wrangler login
   ```

### Staging Deployment

```bash
# Build and deploy to staging
make cf-deploy-staging

# Or manually:
make build-staging
wrangler pages deploy public --project-name=ant-build-staging
```

**Staging URLs:**
- Site: https://staging.ant.build
- API: https://api-staging.ant.build

### Production Deployment

```bash
# Build and deploy to production
make cf-deploy-production

# Or manually:
make build-production
wrangler pages deploy public --project-name=ant-build --branch=main
```

**Production URLs:**
- Site: https://ant.build
- API: https://api.ant.build

### Cloudflare Pages Setup (First Time)

1. Create a new Pages project in Cloudflare Dashboard
2. Connect your Git repository OR use direct uploads
3. Configure build settings:
   - **Build command**: `make build-production`
   - **Build output directory**: `public`
   - **Root directory**: `frontend`

4. Set environment variables (optional):
   - `HUGO_VERSION`: `0.123.7`

### GitHub Actions (CI/CD)

Create `.github/workflows/deploy-frontend.yml`:

```yaml
name: Deploy Frontend

on:
  push:
    branches: [main]
    paths:
      - 'frontend/**'
  pull_request:
    branches: [main]
    paths:
      - 'frontend/**'

jobs:
  build:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: frontend
    steps:
      - uses: actions/checkout@v4

      - name: Setup Hugo
        uses: peaceiris/actions-hugo@v2
        with:
          hugo-version: '0.123.7'
          extended: true

      - name: Build
        run: make build-production

      - name: Test
        run: make test

      - name: Deploy to Cloudflare Pages
        if: github.ref == 'refs/heads/main'
        uses: cloudflare/pages-action@v1
        with:
          apiToken: ${{ secrets.CLOUDFLARE_API_TOKEN }}
          accountId: ${{ secrets.CLOUDFLARE_ACCOUNT_ID }}
          projectName: ant-build
          directory: frontend/public
          gitHubToken: ${{ secrets.GITHUB_TOKEN }}
```

Required secrets:
- `CLOUDFLARE_API_TOKEN`: API token with Pages permissions
- `CLOUDFLARE_ACCOUNT_ID`: Your Cloudflare account ID

## Customization

### Prompt Builder

The prompt builder (`/js/prompt-builder.js`) handles:
- Prompt caching in localStorage
- Authentication flow with email magic links and OAuth
- Submission to the gateway API
- Redirect to provisioning page

### Jobs Page

The jobs page (`/js/jobs.js`) includes:
- Client-side search and filtering
- Sample job data (replace with API call in production)
- Department, type, and location filters

### Environment Variables

The gateway URL is configured per environment:
- **Staging**: `https://api-staging.ant.build`
- **Production**: `https://api.ant.build`

This is set via Hugo's environment config system in `config/<env>/hugo.toml`.

## Docker

```dockerfile
FROM hugomods/hugo:exts as builder
WORKDIR /app
COPY . .
RUN make build-production

FROM nginx:alpine
COPY --from=builder /app/public /usr/share/nginx/html
```

## License

Copyright (c) 2024-2025 Antinvestor. All rights reserved.
