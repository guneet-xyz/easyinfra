# testdata

Test fixtures for easyinfra unit and integration tests.

## Structure

- `infra/` — minimal valid infra repo with 2 apps (alpha, beta)
  - `infra.yaml` — config with kubeContext: test-ctx
  - `charts/alpha/` — minimal Helm chart
  - `charts/beta/` — minimal Helm chart
  - `values-shared.yaml` — shared values
- `bin/` — fake helm and kubectl binaries for tests
  - `helm` — returns canned responses; logs to $HELM_LOG if set
  - `kubectl` — returns canned responses; current-context returns "test-ctx"

## Usage in Tests

Prepend `testdata/bin` to PATH to intercept helm/kubectl calls:

    PATH := filepath.Join(projectRoot, "testdata/bin") + ":" + os.Getenv("PATH")

Or set in test setup:

    t.Setenv("PATH", "testdata/bin:"+os.Getenv("PATH"))
