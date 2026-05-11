# T9 - pkg/repo learnings

- FakeRunner key format: `name arg1 arg2 ...` (space-joined). Default applies when no key matches.
- Git pull `--ff-only` stderr varies: detect both "not fast-forward" and "Not possible to fast-forward".
- Use `os.RemoveAll` then `os.MkdirAll(filepath.Dir(...))` before git clone to support `--force` overwrite.
- Coverage achieved: 90.9% via 13 tests covering happy paths, error paths, and edge cases.
