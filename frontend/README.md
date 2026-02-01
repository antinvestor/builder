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
# Development server with live reload
hugo server -D

# Build for production
hugo --gc --minify

# Build with specific base URL
hugo --gc --minify -b https://ant.build
```

## Project Structure

```
frontend/
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
└── hugo.toml        # Hugo configuration
```

## Configuration

Key configuration in `hugo.toml`:

- `params.gatewayURL`: API endpoint for the ant.build gateway
- `params.enablePromptBuilder`: Toggle prompt builder feature
- `params.enableJobs`: Toggle jobs page
- `params.enableBlog`: Toggle blog section

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

## Deployment

### Static Hosting

Build the site and deploy to any static host:

```bash
hugo --gc --minify
# Deploy the 'public' directory
```

### Docker

```dockerfile
FROM hugomods/hugo:exts as builder
WORKDIR /app
COPY . .
RUN hugo --gc --minify

FROM nginx:alpine
COPY --from=builder /app/public /usr/share/nginx/html
```

### GitHub Pages

```yaml
# .github/workflows/deploy.yml
name: Deploy
on:
  push:
    branches: [main]
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: peaceiris/actions-hugo@v2
      - run: hugo --gc --minify
      - uses: peaceiris/actions-gh-pages@v3
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}
          publish_dir: ./public
```

## License

Copyright (c) 2024-2025 Antinvestor. All rights reserved.
