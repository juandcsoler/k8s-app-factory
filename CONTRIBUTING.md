# Contributing to K8s Core Operator

Thank you for your interest in contributing to K8s Core Operator! This document provides guidelines and instructions for contributing.

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## How to Contribute

### Reporting Bugs

If you find a bug, please open an issue on GitHub with:

1. A clear, descriptive title.
2. Steps to reproduce the problem.
3. Expected vs. actual behavior.
4. Your environment (K8s version, Go version, OS).

### Suggesting Features

Feature requests are welcome. Please open an issue describing:

1. The problem you are trying to solve.
2. Your proposed solution.
3. Any alternatives you have considered.

### Submitting Pull Requests

1. **Fork** the repository and create your branch from `main`.
2. **Install** dependencies:
   ```bash
   go mod download
   ```
3. **Make your changes**, ensuring they follow the project's coding style.
4. **Add or update tests** as needed.
5. **Run the full test suite** to ensure nothing is broken:
   ```bash
   make test
   make lint
   ```
6. **Commit** your changes with clear, descriptive messages following [Conventional Commits](https://www.conventionalcommits.org/).
7. **Push** to your fork and submit a Pull Request.

### Development Setup

```bash
# Clone your fork
git clone https://github.com/<your-user>/platform-k8s-core-operator.git
cd platform-k8s-core-operator

# Install CRDs into a Kind cluster
make install

# Run the operator locally
ENABLE_WEBHOOKS=false make run

# Run tests
make test
```

### Code Style

- Follow standard Go conventions (`gofmt`, `goimports`).
- All comments and documentation must be in **English**.
- All source files must include the Apache 2.0 license header (see `hack/boilerplate.go.txt`).
- Run `make lint` before submitting.

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).
