// =============================================================================
// BAML Generator Configuration
// =============================================================================
// Configures code generation for the BAML client.
// Run `baml-cli generate` to generate the client code.
// =============================================================================

// Generate Go client
generator GoClient {
    output_type "go"

    // Where the generated code will be saved (relative to baml_src/)
    output_dir "../baml_client"

    // Version of runtime to generate code for
    version "0.217.0"

    // Go module name for the generated client
    client_package_name "github.com/antinvestor/builder/baml_client"

    // Commands to run after code generation
    on_generate "gofmt -w . && goimports -w . && go mod tidy"
}
