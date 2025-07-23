# Dockswap Config Directory Discovery Spec

## Purpose
Implement a robust, testable mechanism to determine the most relevant configuration directory for Dockswap, supporting user overrides and standard locations.

## Preference Order
1. If `--config` is provided, use that path (absolute or relative).
2. If not, check for `./dockswap-cfg` in the current working directory.
3. If not, check for `$HOME/.config/dockswap-cfg`.
4. If not, check for `/etc/dockswap-cfg/`.
5. If none found, return an error.

## Success Criteria
- Returns the correct config directory path according to the above order.
- Honors the `--config` override for easy unit testing.
- Returns an error if no config directory is found.
- Is cross-platform (handles $HOME correctly).
- Is easy to unit test (can inject flags and environment).

## Scope
- Implement as a function (e.g., `FindConfigDir(flags GlobalFlags) (string, error)`).
- Used by CLI to load configs.
- Optionally, check for directory existence (recommended for robustness).
- Unit tests for all preference cases and error case.

## Technical Considerations
- Use `os.Stat` or `os.IsNotExist` to check for directory existence.
- Use `os.UserHomeDir()` for `$HOME`.
- Accept both absolute and relative paths for `--config`.
- Should not panic; always return error if not found.
- Should be easily mockable for tests (e.g., allow override of `os.Stat` and `os.UserHomeDir` if needed).

## Out of Scope
- Loading or parsing config files themselves.
- Creating config directories if missing.
- UI/CLI prompts for missing configs.

## Testability
- Unit tests must cover:
  - `--config` provided (absolute and relative)
  - Only `./dockswap-cfg` exists
  - Only `$HOME/.config/dockswap-cfg` exists
  - Only `/etc/dockswap-cfg/` exists
  - None exist (error)
- Tests should not require actual files on disk (can use temp dirs and env overrides). 