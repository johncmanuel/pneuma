
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

## Building

See the section, `Setting up the environment`, in the README.md for details

## Styling

See the section, `Formatting`, in the README.md for the commands

Some general styling guidelines:
- TypeScript
    - Avoid the `any` type if possible. If the proper types cannot be found, use `any` as fallback
    - Use type inference as much as possible; do not explicitly state the type or interfaces for variables unless needed for clarity or exports.
    - Use built-in, functional methods for processing arrays and other data structures over loops
    - Prefer `const` over `let` unless state mutation is needed. 
    - Use ternaries or early returns if possible. For ternaries, if the conditions are too long, define additional variables to ensure readability.
    - Use dot notations over bracket notation for readability.
- Go
    - 
