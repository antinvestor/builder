// =============================================================================
// BAML Client Configuration for Feature Service
// =============================================================================
// Defines LLM provider configurations for the feature service.
// =============================================================================

// -----------------------------------------------------------------------------
// Primary Client - Anthropic Claude
// -----------------------------------------------------------------------------

client<llm> Anthropic {
  provider anthropic
  retry_policy DefaultRetry
  options {
    model "claude-sonnet-4-20250514"
    max_tokens 16384
    temperature 0.0
    api_key env.ANTHROPIC_API_KEY
  }
}

// High-quality client for complex tasks
client<llm> AnthropicOpus {
  provider anthropic
  retry_policy AggressiveRetry
  options {
    model "claude-opus-4-20250514"
    max_tokens 16384
    temperature 0.0
    api_key env.ANTHROPIC_API_KEY
  }
}

// Fast client for simple tasks
client<llm> AnthropicHaiku {
  provider anthropic
  retry_policy DefaultRetry
  options {
    model "claude-3-5-haiku-20241022"
    max_tokens 8192
    temperature 0.0
    api_key env.ANTHROPIC_API_KEY
  }
}

// -----------------------------------------------------------------------------
// Fallback Clients
// -----------------------------------------------------------------------------

// OpenAI fallback
client<llm> OpenAI {
  provider openai
  retry_policy DefaultRetry
  options {
    model "gpt-4o"
    max_tokens 16384
    temperature 0.0
    api_key env.OPENAI_API_KEY
  }
}

// Google fallback
client<llm> Google {
  provider google-ai
  retry_policy DefaultRetry
  options {
    model "gemini-2.0-flash"
    max_tokens 8192
    temperature 0.0
    api_key env.GOOGLE_API_KEY
  }
}

// -----------------------------------------------------------------------------
// Retry Configuration
// -----------------------------------------------------------------------------

retry_policy DefaultRetry {
  max_retries 3
  strategy {
    type exponential_backoff
    delay_ms 1000
    max_delay_ms 60000
    multiplier 2.0
  }
}

retry_policy AggressiveRetry {
  max_retries 5
  strategy {
    type exponential_backoff
    delay_ms 500
    max_delay_ms 120000
    multiplier 2.5
  }
}

// -----------------------------------------------------------------------------
// Client Selection Strategy
// -----------------------------------------------------------------------------

// Primary strategy with fallback
client<llm> Primary {
  provider fallback
  retry_policy DefaultRetry
  options {
    strategy [Anthropic, OpenAI]
  }
}

// High-stakes operations (planning, review)
client<llm> HighStakes {
  provider fallback
  retry_policy AggressiveRetry
  options {
    strategy [AnthropicOpus, Anthropic, OpenAI]
  }
}

// Fast operations (commit messages, simple transformations)
client<llm> Fast {
  provider fallback
  retry_policy DefaultRetry
  options {
    strategy [AnthropicHaiku, Google]
  }
}
