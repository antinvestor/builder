---
title: "Comparing LLM Providers: Why We Use Multiple Models"
date: 2026-01-20
author: "The ant.build Team"
description: "A deep dive into how we use Claude, GPT-4, and Gemini together to deliver the best code generation results."
icon: "🧠"
category: "Engineering"
tags: ["AI", "LLM", "Engineering", "Architecture"]
readingTime: 12
---

When building an AI code generation platform, one of the most important decisions is which language model to use. After extensive testing, we decided not to choose just one—instead, we built a multi-LLM pipeline that leverages the strengths of different models.

## The Challenge

Each LLM has different strengths:

- **Claude** excels at understanding complex requirements and generating well-structured code with excellent documentation
- **GPT-4** is strong at creative problem-solving and handling edge cases
- **Gemini** offers excellent performance for straightforward code generation tasks

Rather than forcing users to choose, we wanted to use the right model for each task automatically.

## Our Multi-LLM Architecture

### BAML Orchestration

We use [BAML](https://docs.boundaryml.com/) (Boundary AI Markup Language) to orchestrate our LLM pipeline. BAML provides:

- **Structured outputs**: Guaranteed JSON/schema compliance
- **Retry logic**: Automatic retries with fallback providers
- **Validation**: Built-in output validation
- **Type safety**: Full TypeScript/Go type generation

Here's a simplified example of our pipeline:

```baml
function NormalizeSpec {
  input: RawSpecification
  output: NormalizedSpecification

  client: Claude
  fallback: [GPT4, Gemini]

  prompt #"
    Analyze this feature specification and extract:
    - Core requirements
    - Technical constraints
    - Edge cases to handle
    - Dependencies needed

    Specification: {{ input }}
  "#
}
```

### Provider Fallback

If Claude is unavailable or rate-limited, we automatically fall back to GPT-4, then Gemini. This ensures 99.9% availability without user intervention.

```go
// Simplified fallback logic
providers := []LLMProvider{Claude, GPT4, Gemini}

for _, provider := range providers {
    result, err := provider.Generate(prompt)
    if err == nil {
        return result
    }
    log.Warn("Provider failed, trying next", "provider", provider.Name())
}
```

### Task-Specific Routing

Some tasks are routed to specific models based on their strengths:

| Task | Primary | Fallback |
|------|---------|----------|
| Specification analysis | Claude Opus | GPT-4 |
| Code generation | Claude Sonnet | GPT-4 |
| Test generation | Claude Sonnet | Gemini |
| Documentation | Claude Opus | GPT-4 |
| Quick fixes | Gemini Flash | Claude Haiku |

## Rate Limiting & Cost Optimization

### Token Bucket Algorithm

We implement per-provider rate limiting using a token bucket algorithm:

```go
type RateLimiter struct {
    tokens     float64
    maxTokens  float64
    refillRate float64
    lastRefill time.Time
}

func (rl *RateLimiter) Allow() bool {
    rl.refill()
    if rl.tokens >= 1 {
        rl.tokens--
        return true
    }
    return false
}
```

### Cost Management

By routing simpler tasks to faster, cheaper models (Gemini Flash, Claude Haiku), we reduce costs while maintaining quality where it matters.

**Average cost per feature generation:**
- Simple feature: ~$0.05
- Medium complexity: ~$0.15
- Complex feature: ~$0.40

And remember—this cost is on us. Code generation is free for users!

## Quality Assurance

### Output Validation

Every generated output is validated against our schema:

```go
func validateCodeOutput(output CodeOutput) error {
    // Check syntax
    if err := parseSyntax(output.Language, output.Code); err != nil {
        return fmt.Errorf("syntax error: %w", err)
    }

    // Check for security issues
    if issues := securityScan(output.Code); len(issues) > 0 {
        return fmt.Errorf("security issues: %v", issues)
    }

    // Verify tests compile
    if err := compileTests(output.Tests); err != nil {
        return fmt.Errorf("test compilation failed: %w", err)
    }

    return nil
}
```

### Automated Testing

Generated code is tested in sandboxed environments before delivery:

1. Unit tests run in isolated containers
2. Integration tests verify API contracts
3. Security scans check for vulnerabilities
4. Linters ensure code style compliance

## Results

Since implementing our multi-LLM pipeline:

- **99.9% availability** (up from 95% with single provider)
- **40% cost reduction** through intelligent routing
- **15% quality improvement** by using the best model for each task
- **2x faster generation** with parallel model queries

## Lessons Learned

1. **Don't put all eggs in one basket**: Provider outages happen. Have fallbacks ready.

2. **Match models to tasks**: Expensive models aren't always better. Route intelligently.

3. **Validate everything**: LLMs can hallucinate. Always validate outputs.

4. **Monitor costs**: Token usage can spike unexpectedly. Implement budgets and alerts.

5. **Keep humans in the loop**: AI generates, humans review. This catches subtle issues.

## What's Next?

We're constantly evaluating new models as they're released. When a new model shows promise, we add it to our pipeline and run comparative benchmarks.

Want to see our multi-LLM pipeline in action? [Try ant.build for free](/#prompt-builder).

---

Interested in the technical details? Check out our [architecture documentation](/docs/architecture/) or join our [Discord](https://discord.gg/antinvestor) for discussions.
