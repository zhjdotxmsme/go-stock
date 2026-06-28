// Package strategy provides a library of built-in trading analysis strategies.
// Each strategy registers itself via init() and provides a Prompt that gets injected
// into the multi-agent LLM synthesis stage to shape the analytical perspective.
//
// To add a new strategy:
//  1. Create a new file in this package (e.g., batch1_my_strategy.go)
//  2. Define a Prompt const with the LLM analysis perspective
//  3. Call Register(&Strategy{...}) in an init() function
//
// All strategies in this package are auto-registered when the package is imported
// via blank import in app.go: _ "go-stock/backend/agent/strategy"
package strategy
