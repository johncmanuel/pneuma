## Project Structure

- Desktop application
  - Desktop UI is in `frontend/`
  - Desktop backend is in `internal/`. Most of the backend is found in `internal/desktop` under the package `desktop`.
- Web player
  - Web player UI is in `web/`
- Dashboard
  - Dashboard UI is in `dashboard/`
- Server
  - Server code is in `internal/`
- Database
  - Database code is in `internal/store/sqlite`
- Shared
  - Shared code between different components of the project is in `packages/`
- Build
  - Build outputs are found in `build/bin/`
  - `build/darwin` and `build/windows` are folders that `wails build` will use when building and packaging MacOS and Windows applications for this project respectively
- Landing page
  - The landing page for the project is in `landing/`

## Package management

Bun is used for package management in Svelte and TypeScript environments. Do not use npm or other Node-based package managers.

## Building

See the section, `Setting up the environment`, in the README.md for details

## Styling

Some general styling guidelines:

- TypeScript
  - Avoid the `any` type if possible. If the proper types cannot be found, use `any` as fallback
  - Use type inference as much as possible; do not explicitly state the type or interfaces for variables unless needed for clarity or exports.
  - Use built-in, functional methods for processing arrays and other data structures over loops
  - Prefer `const` over `let` unless state mutation is needed.
  - Use ternaries or early returns if possible. For ternaries, if the conditions are too long, define additional variables to ensure readability.
  - Use dot notations over bracket notation for readability.
  - Use `Boolean()` instead of `!!` for readability.
  - Avoid empty blocks, especially in `try/catch` statements; use console.log or equivalent to fill in the blocks.
- Go
  - Run `go vet ./...` when making changes to the codebase.
  - Always handle errors explicitly; never ignore them with `_` unless there's a deliberate, commented reason. Prefer early returns over nested `if` blocks.
  - Use `camelCase` for unexported identifiers and `PascalCase` for exported ones. Any acronyms should be all caps.
  - Group `const` and `var` variables into a single declaration block using parentheses.
  - Avoid using naked returns, unless the function is very short.
- Svelte
  - Keep components small and single-purpose
  - Avoid logic in templates beyond simple ternaries or #each/#if blocks
  - Prefer scoped styles. Use global styles when absolutely necessary
- HTML
  - Keep markup free of inline styles; all styling belongs in CSS
  - Always include alt attributes on images
  - Prioritize accessibility via ARIA
  - Boolean attributes should be written without a value

### Formatting

Run `bun fmt` to format TypeScript, Svelte, and Go code.

Run them after finalizing changes.

### Linting

Run `bun lint` to lint TypeScript, Svelte, and Go code.

Run `bun knip` to check for unused TypeScript and Svelte code, dependencies, and exports.

Run them after finalizing changes.

### Chrome Devtools MCP

Utilize the following commands to start, verify and log outputs of, and stop the server. Always ensure to place any logs in `./tmp`. Assume the current directory is at root, `./`

1. Start: `nohup bun server > "tmp/server-run.log" 2>&1 & disown` (note: keep the .log filename unique)
2. Check processes: `pgrep -af "bun server|go run ./cmd/server|/tmp/go-build|/cmd/server"`
3. Verify port: `ss -ltnp | rg 8989`
4. Stop: `pkill -f "bun server|go run ./cmd/server|/tmp/go-build.*exe/server"`

The default address is http://127.0.0.1:8989. The web player can be found at http://127.0.0.1:8989/player. The dashboard can be found at http://127.0.0.1:8989/dashboard.

When testing changes, be sure to perform the registration process (use any username/password) and test the features of the project. Be as rigorous as possible.

Once the testing and verification are done, run the Stop command and report the results.
