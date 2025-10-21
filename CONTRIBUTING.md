# Contributing to boGO

Thank you for your interest in contributing to boGO! We welcome contributions from the community and are excited to see what you'll bring to the project.

## Quick Start for Contributors

1. **Fork** the repository
2. **Clone** your fork locally
3. **Create** a feature branch
4. **Make** your changes
5. **Test** thoroughly
6. **Submit** a pull request

## Ways to Contribute

### Bug Reports
- Use the [GitHub Issues](https://github.com/WithBogo/boGO/issues) to report bugs
- Include clear steps to reproduce
- Provide system information (OS, Go version, etc.)
- Share relevant error messages or logs

### Feature Requests
- Open an issue with the `enhancement` label
- Describe the problem you're trying to solve
- Explain how your feature would help users
- Consider implementation complexity

### Code Contributions
- **Templates**: Improve or add new code generation templates
- **Core Logic**: Enhance SQL parsing, file generation, or architecture
- **Documentation**: Update guides, examples, or API docs
- **Testing**: Add test cases or improve test coverage
- **Performance**: Optimize code generation speed or output quality

### Documentation
- Fix typos or improve clarity
- Add new examples or use cases
- Create tutorials or guides
- Translate documentation

## Development Setup

### Prerequisites
- Go 1.22 or later
- Git
- PostgreSQL (for testing generated services)

### Local Development
```bash
# Clone your fork
git clone https://github.com/YOUR-USERNAME/boGO.git
cd boGO

# Install dependencies
go mod tidy

# Run tests
go test ./...

# Test code generation
go run . test-service test_schema.sql
```

### Testing Your Changes
```bash
# Test with different SQL schemas
go run . user-service examples/user_schema.sql
go run . ecommerce-api examples/ecommerce_schema.sql

# Test generated service
cd user-service
docker-compose up -d
curl http://localhost:8080/health
```

## 📝 Code Style Guidelines

### Go Code Standards
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `go fmt` for formatting
- Run `go vet` and `golint`
- Add comments for exported functions
- Keep functions small and focused

### Template Standards
- Use clear variable names in templates
- Add comments explaining complex logic
- Follow Go naming conventions in generated code
- Ensure generated code passes linting

### Commit Messages
Use conventional commit format:
```
type(scope): description

Examples:
feat(templates): add JWT authentication middleware
fix(parser): handle nullable foreign keys correctly
docs(readme): update quick start guide
test(generator): add SQL parsing edge cases
```

Types: `feat`, `fix`, `docs`, `test`, `refactor`, `style`, `chore`

## 🔍 Pull Request Process

### Before Submitting
- [ ] Code follows project style guidelines
- [ ] Tests pass locally
- [ ] Generated code compiles and runs
- [ ] Documentation updated if needed
- [ ] Commit messages follow convention

### PR Description Template
```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Documentation update
- [ ] Performance improvement
- [ ] Code refactoring

## Testing
- [ ] Tested with multiple SQL schemas
- [ ] Generated services run successfully
- [ ] All tests pass

## Screenshots (if applicable)
Add screenshots of generated code or API responses
```

### Review Process
1. **Automated checks** must pass (CI/CD)
2. **Code review** by maintainers
3. **Testing** on different environments
4. **Merge** after approval

## 🧪 Testing Guidelines

### Unit Tests
- Test SQL parsing edge cases
- Test template generation logic
- Mock external dependencies
- Aim for >80% coverage

### Integration Tests
- Test complete code generation flow
- Verify generated services work
- Test Docker containers
- Test database migrations

### Manual Testing
- Generate services from various schemas
- Test REST API endpoints
- Verify Docker setup works
- Check generated documentation

## 📁 Project Structure

Understanding the codebase:

```
boGO/
├── main.go                 # CLI entry point
├── sql-parser.go          # SQL schema parsing
├── directory-gen.go       # Directory structure creation
├── file-generator.go      # File generation orchestration
├── *-templates.go         # Template generation logic
├── template-loader.go     # Template loading system
├── templates/             # Code generation templates
│   ├── application/       # App layer templates
│   ├── domain/           # Domain layer templates
│   ├── interactor/       # Business logic templates
│   ├── repository/       # Data layer templates
│   ├── rest/             # API layer templates
│   ├── base/             # Base files (Docker, etc.)
│   └── migration/        # DB migration templates
└── web/                  # Project website
```

## 🎯 Contribution Ideas

### Easy (Good First Issues)
- Fix typos in documentation
- Add new SQL data type support
- Improve error messages
- Add code comments

### Medium
- Add new template features
- Improve SQL parsing
- Add new architectural patterns
- Create new examples

### Advanced
- Add new database support (MySQL, MongoDB)
- Implement GraphQL generation
- Add microservice orchestration
- Create IDE plugins

## 🏷️ Issue Labels

- `good first issue` - Easy for newcomers
- `help wanted` - Community help needed
- `bug` - Something isn't working
- `enhancement` - New feature request
- `documentation` - Documentation improvement
- `templates` - Template-related changes
- `parser` - SQL parsing issues
- `architecture` - Architecture improvements

## 🤝 Community Guidelines

### Be Respectful
- Use welcoming and inclusive language
- Respect different viewpoints and experiences
- Accept constructive criticism gracefully
- Focus on what's best for the community

### Be Collaborative
- Help others learn and grow
- Share knowledge and resources
- Provide constructive feedback
- Celebrate others' contributions

### Be Professional
- Keep discussions on-topic
- Avoid personal attacks or harassment
- Follow the project's code of conduct
- Represent the project positively

## 💬 Getting Help

- **GitHub Issues**: For bugs and feature requests
- **GitHub Discussions**: For questions and ideas
- **Documentation**: Check [WithBogo.dev](https://WithBogo.dev)
- **Examples**: Look at existing templates and generated code

## 🎉 Recognition

Contributors will be:
- Listed in project acknowledgments
- Mentioned in release notes
- Invited to become maintainers (for significant contributions)
- Featured on the project website

## 📜 Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you agree to uphold this code.

---

**Thank you for contributing to boGO! Together, we're making Go development obviously better.** 🚀