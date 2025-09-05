# GitHub Copilot Guidelines

This is a Go based repository that is a MCP server for OpenFoodFacts.

## Code Standards

### Development Flow

- Test: `script/test`
- Lint: `script/lint`
- Build: `script/build`

> Note: `script/build --single-target` can be used when iterating on changes rapidly as it will only build the current target which is faster than building all targets.

## Repository Structure

- `cmd/*`: Main entry point
- `internal/`: Logic related to the core functionality of the MCP server
- `script/`: Scripts for building, testing, and releasing the project
- `.github/`: GitHub Actions workflows for CI/CD
- `vendor/`: Vendor directory for Go modules (committed to the repository for reproducibility)

## Key Guidelines

1. Follow Go best practices and idiomatic patterns
2. Maintain existing code structure and organization
3. Use dependency injection patterns where appropriate
4. Write unit tests for new functionality. Use table-driven unit tests when possible. Use `testify` for assertions.
5. When responding to code refactoring suggestions, function suggestions, or other code changes, please keep your responses as concise as possible. We are capable engineers and can understand the code changes without excessive explanation. If you feel that a more detailed explanation is necessary, you can provide it, but keep it concise.
6. When suggesting code changes, always opt for the most maintainable approach. Try your best to keep the code clean and follow DRY principles. Avoid unnecessary complexity and always consider the long-term maintainability of the code.
7. When writing unit tests, always strive for 100% code coverage where it makes sense. Try to consider edge cases as well.
8. Always strive to keep the codebase clean and maintainable. Avoid unnecessary complexity and always consider the long-term maintainability of the code.
9. Always strive for the highest level of code coverage with unit tests where possible.
10. Keep your responses and summary messages short, and very concise. Don't over explain unless asked to in your "what I did" summaries.
